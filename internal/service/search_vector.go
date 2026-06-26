package service

import (
	"context"
	"fmt"

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
	tx.Find(&embedded)

	// 无 embedding 的政策补充
	need := s.cfg.MaxResults - len(embedded)
	if need > 0 {
		var noEmb []model.Policy
		s.db.WithContext(ctx).Where("status = ? AND embedding IS NULL", model.PolicyPublished).
			Order("published_at DESC").Limit(need).Find(&noEmb)
		embedded = append(embedded, noEmb...)
	}

	// AI 精排
	analysis, rankedIDs, effect := s.aiSvc.AnalyzeSearchResults(ctx, query, ent, embedded)

	// 按 rankedIDs 重排
	if len(rankedIDs) > 0 {
		ranked := make([]model.Policy, 0, len(embedded))
		seen := make(map[uint]bool, len(rankedIDs))
		for _, id := range rankedIDs {
			for _, p := range embedded {
				if p.ID == id && !seen[id] {
					ranked = append(ranked, p)
					seen[id] = true
					break
				}
			}
		}
		for _, p := range embedded {
			if !seen[p.ID] {
				ranked = append(ranked, p)
			}
		}
		embedded = ranked
	}

	return &SearchResult{Policies: embedded, Analysis: analysis, RankedIDs: rankedIDs, Effect: effect}, nil
}
