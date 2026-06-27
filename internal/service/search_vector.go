package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// VectorSearch implements PolicySearch via embedding vector similarity matching.
type VectorSearch struct {
	embedClient *aiclient.EmbeddingClient
	aiSvc       *AIService
	expander    *QueryExpander
	hydeGen     *HyDEGenerator
	db          *gorm.DB
	cfg         config.SearchConfig
}

func NewVectorSearch(embedClient *aiclient.EmbeddingClient, aiSvc *AIService, expander *QueryExpander, hydeGen *HyDEGenerator, db *gorm.DB, cfg config.SearchConfig) *VectorSearch {
	return &VectorSearch{embedClient: embedClient, aiSvc: aiSvc, expander: expander, hydeGen: hydeGen, db: db, cfg: cfg}
}

func (s *VectorSearch) Search(ctx context.Context, userID uint, query string) (*SearchResult, error) {
	ent, err := s.aiSvc.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrForbidden.WithMsg("无权访问")
	}

	// MQE: 扩展查询
	queries := []string{query}
	if s.cfg.Vector.MQE.Enabled && s.expander != nil {
		if expanded, expandErr := s.expander.Expand(ctx, query); expandErr != nil {
			slog.Warn("MQE expand failed, using original query", "error", expandErr)
		} else {
			queries = expanded
		}
	}

	// HyDE: 并行生成假设文档，追加到查询列表
	if s.cfg.Vector.HyDE.Enabled && s.hydeGen != nil {
		hydeDocs := make([]string, len(queries))
		g, hydeCtx := errgroup.WithContext(ctx)
		for i, q := range queries {
			g.Go(func() error {
				doc, err := s.hydeGen.Generate(hydeCtx, q)
				if err != nil {
					slog.Warn("HyDE generate failed", "query", q, "error", err)
					return nil
				}
				hydeDocs[i] = doc
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			slog.Warn("HyDE generation aborted", "error", err)
		}
		appended := 0
		for _, doc := range hydeDocs {
			if doc != "" {
				queries = append(queries, doc)
				appended++
			}
		}
		if appended == 0 {
			slog.Warn("HyDE generated zero documents, using original queries only", "expected", len(hydeDocs))
		}
	}

	vcfg := s.cfg.Vector
	topK := vcfg.TopK
	if topK <= 0 {
		topK = s.cfg.MaxResults
	}

	// 并行向量检索
	var mu sync.Mutex
	var allResults [][]model.Policy
	var successCount int

	g, gctx := errgroup.WithContext(ctx)
	for _, q := range queries {
		g.Go(func() error {
			searchText := fmt.Sprintf("企业：行业=%s，规模=%s，地址=%s。%s", ent.Industry, ent.Scale, ent.Address, q)
			emb, err := s.embedClient.Embed(gctx, searchText)
			if err != nil {
				slog.Warn("embed query failed", "query", q, "error", err)
				return nil // 容错：忽略失败
			}
			vecStr := model.PGVector(emb).String()

			var policies []model.Policy
			tx := s.db.WithContext(gctx).
				Where("status = ? AND embedding IS NOT NULL", model.PolicyPublished).
				Order(fmt.Sprintf("embedding <=> '%s'", vecStr)).
				Limit(topK)
			if vcfg.MinScore > 0 {
				tx = tx.Where(fmt.Sprintf("embedding <=> '%s' < %f", vecStr, 1.0-vcfg.MinScore))
			}
			if err := tx.Find(&policies).Error; err != nil {
				slog.Warn("vector query failed", "query", q, "error", err)
				return nil // 容错
			}
			mu.Lock()
			allResults = append(allResults, policies)
			successCount++
			mu.Unlock()
			return nil
		})
	}
	g.Wait()

	// 全部并行查询失败
	if successCount == 0 {
		return nil, errcode.ErrAIService.WithMsg("向量检索失败，请稍后重试")
	}

	// RRF 融合
	var embedded []model.Policy
	if len(allResults) > 0 {
		embedded = rrfFusion(allResults, vcfg.MQE.RRFK, topK)
	}

	// 无 embedding 的政策补充
	need := s.cfg.MaxResults - len(embedded)
	if need > 0 {
		var noEmb []model.Policy
		if err := s.db.WithContext(ctx).Where("status = ? AND embedding IS NULL", model.PolicyPublished).
			Order("published_at DESC").Limit(need).Find(&noEmb).Error; err != nil {
			slog.Warn("no-embedding query failed", "error", err)
		}
		embedded = append(embedded, noEmb...)
	}

	// AI 分析
	analysisResult, err := s.aiSvc.AnalyzeResults(ctx, query, ent, embedded)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Policies: embedded,
		Analysis: analysisResult.Text,
		Found:    analysisResult.Found,
		Effect:   analysisResult.Effect,
	}, nil
}

// rrfFusion performs Reciprocal Rank Fusion on multiple ranked lists.
// k is the RRF constant (typically 60), topK limits the final output.
func rrfFusion(results [][]model.Policy, k float64, topK int) []model.Policy {
	scores := make(map[uint]float64)
	firstSeen := make(map[uint]model.Policy)

	for _, list := range results {
		for rank, p := range list {
			scores[p.ID] += 1.0 / (k + float64(rank+1))
			if _, exists := firstSeen[p.ID]; !exists {
				firstSeen[p.ID] = p
			}
		}
	}

	type scored struct {
		id    uint
		score float64
	}
	var ranked []scored
	for id, s := range scores {
		ranked = append(ranked, scored{id: id, score: s})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	if topK > 0 && len(ranked) > topK {
		ranked = ranked[:topK]
	}

	result := make([]model.Policy, 0, len(ranked))
	for _, r := range ranked {
		result = append(result, firstSeen[r.id])
	}
	return result
}
