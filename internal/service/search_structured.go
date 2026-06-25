package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// StructuredSearch implements PolicySearch via AI structured query extraction + JSONB fuzzy matching.
type StructuredSearch struct {
	aiSvc *AIService
	db    *gorm.DB
	cfg   config.SearchConfig
}

func NewStructuredSearch(aiSvc *AIService, db *gorm.DB, cfg config.SearchConfig) *StructuredSearch {
	return &StructuredSearch{aiSvc: aiSvc, db: db, cfg: cfg}
}

func (s *StructuredSearch) Search(ctx context.Context, userID uint, query string) ([]model.Policy, error) {
	// 1. 获取用户画像
	ent, err := s.aiSvc.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrForbidden.WithMsg("无权访问")
	}

	// 2. AI 转为结构化查询条件
	criteria, err := s.analyzeQuery(ctx, query, ent)
	if err != nil {
		slog.Warn("ai analyze query failed", "error", err)
		return nil, errcode.ErrAIService.WithMsg("搜索分析失败")
	}

	// 3. JSONB 模糊匹配 + 评分排序
	policies, err := s.searchPolicies(ctx, criteria)
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (s *StructuredSearch) analyzeQuery(ctx context.Context, query string, ent *model.Enterprise) (*SearchCriteria, error) {
	userMsg := fmt.Sprintf(
		"企业信息：行业=%s、规模=%s、地址=%s\n用户搜索：%s\n\n"+
			"请从用户的描述中提取关键条件，严格按照以下 JSON 格式返回：\n"+
			`{"applicable_industries":["匹配的行业关键词"],"applicable_scales":["匹配的企业规模关键词"],"applicable_status":"适用状态","subsidy_types":["补贴类型"],"applicable_region":"区域关键词"}`+"\n"+
			"如果未提及某个字段，用空值表示（字符串用空字符串，数组用空数组）。",
		ent.Industry, ent.Scale, ent.Address, query,
	)
	return chatAndParse[SearchCriteria](s.aiSvc, ctx, "search", s.aiSvc.prompts.search, userMsg, "AI搜索分析失败")
}

func (s *StructuredSearch) searchPolicies(ctx context.Context, criteria *SearchCriteria) ([]model.Policy, error) {
	type scoreCase struct {
		field   string
		keyword string
	}
	var scores []scoreCase

	for _, kw := range criteria.ApplicableIndustries {
		if kw != "" {
			scores = append(scores, scoreCase{"applicable_industries", kw})
		}
	}
	for _, kw := range criteria.ApplicableScales {
		if kw != "" {
			scores = append(scores, scoreCase{"applicable_scales", kw})
		}
	}
	if criteria.ApplicableStatus != "" {
		scores = append(scores, scoreCase{"applicable_status", criteria.ApplicableStatus})
	}
	for _, kw := range criteria.SubsidyTypes {
		if kw != "" {
			scores = append(scores, scoreCase{"subsidy_type", kw})
		}
	}
	if criteria.ApplicableRegion != "" {
		scores = append(scores, scoreCase{"applicable_region", criteria.ApplicableRegion})
	}

	if len(scores) == 0 {
		// 无条件，返回最近发布
		var policies []model.Policy
		s.db.Where("status = ?", model.PolicyPublished).Order("published_at DESC").Limit(s.cfg.MaxResults).Find(&policies)
		return policies, nil
	}

	// 用 raw SQL 避免 ORM 复杂嵌套
	query := `SELECT id, title, target_role, requirements, start_date, end_date, status, published_at, extracted_fields
		FROM policies WHERE status = $1 AND (`
	args := []any{string(model.PolicyPublished)}

	var orParts []string
	for _, sc := range scores {
		orParts = append(orParts, fmt.Sprintf("extracted_fields->>'%s' ILIKE '%%%%%s%%%%'", sc.field, sc.keyword))
	}
	query += strings.Join(orParts, " OR ") + ")"

	// 排序：命中数降序 + 发布时间降序
	var scoreParts []string
	for _, sc := range scores {
		scoreParts = append(scoreParts, fmt.Sprintf("CASE WHEN extracted_fields->>'%s' ILIKE '%%%%%s%%%%' THEN 1 ELSE 0 END", sc.field, sc.keyword))
	}
	query += " ORDER BY (" + strings.Join(scoreParts, " + ") + ") DESC, published_at DESC"
	query += fmt.Sprintf(" LIMIT %d", s.cfg.MaxResults)

	var policies []model.Policy
	if err := s.db.Raw(query, args...).Scan(&policies).Error; err != nil {
		slog.Error("search policies failed", "error", err)
		return nil, errcode.ErrInternal
	}
	return policies, nil
}
