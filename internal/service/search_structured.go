package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// extractedFieldsSubsidiesPath 是 extracted_fields JSONB 中补贴列表的路径。
// 与 model.ExtractedPolicy.Subsidies 的 JSON tag "subsidies" 保持一致。
const extractedFieldsSubsidiesPath = "subsidies"

// StructuredSearch implements PolicySearch via AI structured query extraction + JSONB fuzzy matching.
type StructuredSearch struct {
	aiSvc *AIService
	db    *gorm.DB
	cfg   config.SearchConfig
}

func NewStructuredSearch(aiSvc *AIService, db *gorm.DB, cfg config.SearchConfig) *StructuredSearch {
	return &StructuredSearch{aiSvc: aiSvc, db: db, cfg: cfg}
}

func (s *StructuredSearch) Search(ctx context.Context, userID uint, query string) (*SearchResult, error) {
	// 1. 获取用户画像
	ent, err := s.aiSvc.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			ent = &model.Enterprise{}
			// 载体用户：查询载体画像并填充 ent 用于 AI 分析
			var carrier model.Carrier
			if err := s.db.Where("user_id = ?", userID).First(&carrier).Error; err == nil {
				ent.Industry = carrier.Type
				ent.Scale = carrier.Scale
				ent.Address = carrier.Area
				ent.ID = 1
			}
		} else {
			return nil, errcode.ErrInternal.WithMsg("查询企业信息失败")
		}
	}

	// 2. AI 转为结构化查询条件
	criteria, err := s.analyzeQuery(ctx, query, ent)
	if err != nil {
		slog.Warn("ai analyze query failed", "error", err)
		return nil, errcode.ErrAIService.WithMsg("搜索分析失败")
	}

	// 3. JSONB 模糊匹配
	policies, err := s.searchPolicies(ctx, criteria)
	if err != nil {
		return nil, err
	}

	// 4. AI 精排 + 分析
	policies, analysisResult, err := s.aiSvc.AnalyzeAndRankResults(ctx, query, ent, policies)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Policies: policies,
		Analysis: analysisResult.Text,
		Found:    analysisResult.Found,
		Effect:   analysisResult.Effect}, nil
}

func (s *StructuredSearch) analyzeQuery(ctx context.Context, query string, ent *model.Enterprise) (*SearchCriteria, error) {
	profileStr := enterpriseProfileStr(ent)
	userMsg := fmt.Sprintf("%s用户搜索：%s\n\n"+
		"请从用户的描述中提取关键条件，严格按照以下 JSON 格式返回：\n"+
		`{"applicable_industries":["匹配的行业关键词"],"applicable_scales":["匹配的企业规模关键词"],"applicable_status":"适用状态","subsidy_types":["补贴类型"],"applicable_region":"区域关键词","subsidy_amount_keywords":["金额关键词，如 10万、20万、30万 等"]}`+"\n"+
		"如果未提及某个字段，用空值表示（字符串用空字符串，数组用空数组）。",
		profileStr, query,
	)
	return chatAndParse[SearchCriteria](s.aiSvc, ctx, "search", s.aiSvc.prompts.search, userMsg, "AI搜索分析失败")
}

func (s *StructuredSearch) searchPolicies(ctx context.Context, criteria *SearchCriteria) ([]model.Policy, error) {
	type fieldMatch struct {
		field   string
		keyword string
	}
	var matches []fieldMatch

	for _, kw := range criteria.ApplicableIndustries {
		if kw != "" {
			matches = append(matches, fieldMatch{"applicable_industries", kw})
		}
	}
	for _, kw := range criteria.ApplicableScales {
		if kw != "" {
			matches = append(matches, fieldMatch{"applicable_scales", kw})
		}
	}
	if criteria.ApplicableStatus != "" {
		matches = append(matches, fieldMatch{"applicable_status", criteria.ApplicableStatus})
	}
	for _, kw := range criteria.SubsidyTypes {
		if kw != "" {
			matches = append(matches, fieldMatch{"subsidy_type", kw})
		}
	}
	if criteria.ApplicableRegion != "" {
		matches = append(matches, fieldMatch{"applicable_region", criteria.ApplicableRegion})
	}
	for _, kw := range criteria.SubsidyAmountKeywords {
		if kw != "" {
			matches = append(matches, fieldMatch{"subsidy_amount", kw})
		}
	}

	maxResults := s.cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	if len(matches) == 0 {
		var policies []model.Policy
		if err := s.db.WithContext(ctx).Where("status = ?", model.PolicyPublished).
			Order("published_at DESC").Limit(maxResults).Find(&policies).Error; err != nil {
			slog.Error("search policies failed", "error", err)
			return nil, errcode.ErrInternal
		}
		return policies, nil
	}

	// 参数化关键词，避免 SQL 注入
	var args []any
	var orParts []string
	for _, m := range matches {
		if m.field == "subsidy_amount" {
			orParts = append(orParts, fmt.Sprintf(
				`EXISTS (SELECT 1 FROM jsonb_array_elements(extracted_fields->'%s') AS s WHERE s->>'amount' ILIKE ?)`,
				extractedFieldsSubsidiesPath))
		} else {
			orParts = append(orParts, fmt.Sprintf("extracted_fields->>'%s' ILIKE ?", m.field))
		}
		args = append(args, "%"+m.keyword+"%")
	}
	args = append(args, string(model.PolicyPublished))
	whereClause := "(" + strings.Join(orParts, " OR ") + ") AND status = ?"

	var policies []model.Policy
	tx := s.db.WithContext(ctx).Where(whereClause, args...).Order("published_at DESC").Limit(maxResults)
	if err := tx.Find(&policies).Error; err != nil {
		slog.Error("search policies failed", "error", err)
		return nil, errcode.ErrInternal
	}
	return policies, nil
}
