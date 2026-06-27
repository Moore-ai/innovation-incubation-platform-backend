package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

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
	db          *gorm.DB
	cfg         config.SearchConfig
}

func NewVectorSearch(embedClient *aiclient.EmbeddingClient, aiSvc *AIService, db *gorm.DB, cfg config.SearchConfig) *VectorSearch {
	return &VectorSearch{embedClient: embedClient, aiSvc: aiSvc, db: db, cfg: cfg}
}

func (s *VectorSearch) Search(ctx context.Context, userID uint, query string) (*SearchResult, error) {
	ent, err := s.aiSvc.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrForbidden.WithMsg("无权访问")
	}

	// 用户画像 + query -> embedding
	searchText := fmt.Sprintf("企业：行业=%s，规模=%s，地址=%s。%s", ent.Industry, ent.Scale, ent.Address, query)
	queryEmb, err := s.embedClient.Embed(ctx, searchText)
	if err != nil {
		return nil, errcode.ErrAIService.WithMsg("向量化失败")
	}

	vecStr := model.PGVector(queryEmb).String()
	vcfg := s.cfg.Vector

	// 有 embedding 的政策：向量距离排序
	topK := vcfg.TopK
	if topK <= 0 {
		topK = s.cfg.MaxResults
	}

	var embedded []model.Policy
	tx := s.db.WithContext(ctx).
		Where("status = ? AND embedding IS NOT NULL", model.PolicyPublished).
		Order(fmt.Sprintf("embedding <=> '%s'", vecStr)).
		Limit(topK)
	if vcfg.MinScore > 0 {
		tx = tx.Where(fmt.Sprintf("embedding <=> '%s' < %f", vecStr, 1.0-vcfg.MinScore))
	}
	if err := tx.Find(&embedded).Error; err != nil {
		slog.Error("embedding query failed", "error", err)
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

	// AI 分析（不做重排）
	analysisResult, err := s.aiSvc.AnalyzeResults(ctx, query, ent, embedded)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Policies: embedded,
		Analysis: analysisResult.Text,
		Found:    analysisResult.Found,
		Effect:   analysisResult.Effect}, nil
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
