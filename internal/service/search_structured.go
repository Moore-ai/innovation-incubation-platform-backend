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

func (s *StructuredSearch) Search(ctx context.Context, userID uint, query string) (*SearchResult, error) {
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

	// 3. JSONB 模糊匹配
	policies, err := s.searchPolicies(ctx, criteria)
	if err != nil {
		return nil, err
	}

	// 4. AI 精排 + 分析
	analysis, rankedIDs, effect := s.analyzeResults(ctx, query, ent, policies)

	// 5. 按 AI 输出的 ranked_ids 重排，未列出的放在末尾
	if len(rankedIDs) > 0 {
		ranked := make([]model.Policy, 0, len(policies))
		seen := make(map[uint]bool, len(rankedIDs))
		for _, id := range rankedIDs {
			for _, p := range policies {
				if p.ID == id && !seen[id] {
					ranked = append(ranked, p)
					seen[id] = true
					break
				}
			}
		}
		// 补充未在 ranked_ids 中的政策
		for _, p := range policies {
			if !seen[p.ID] {
				ranked = append(ranked, p)
			}
		}
		policies = ranked
	}

	return &SearchResult{Policies: policies, Analysis: analysis, RankedIDs: rankedIDs, Effect: effect}, nil
}

func (s *StructuredSearch) analyzeQuery(ctx context.Context, query string, ent *model.Enterprise) (*SearchCriteria, error) {
	userMsg := fmt.Sprintf(
		"企业信息：行业=%s、规模=%s、地址=%s\n用户搜索：%s\n\n"+
			"请从用户的描述中提取关键条件，严格按照以下 JSON 格式返回：\n"+
			`{"applicable_industries":["匹配的行业关键词"],"applicable_scales":["匹配的企业规模关键词"],"applicable_status":"适用状态","subsidy_types":["补贴类型"],"applicable_region":"区域关键词","subsidy_amount_keywords":["金额关键词，如 10万、20万、30万 等"]}`+"\n"+
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
	for _, kw := range criteria.SubsidyAmountKeywords {
		if kw != "" {
			scores = append(scores, scoreCase{"subsidy_amount", kw})
		}
	}

	maxResults := s.cfg.MaxResults
	if maxResults <= 0 {
		maxResults = 10
	}

	if len(scores) == 0 {
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
	for _, sc := range scores {
		if sc.field == "subsidy_amount" {
			orParts = append(orParts, `EXISTS (SELECT 1 FROM jsonb_array_elements(extracted_fields->'subsidies') AS s WHERE s->>'amount' ILIKE ?)`)
		} else {
			orParts = append(orParts, fmt.Sprintf("extracted_fields->>'%s' ILIKE ?", sc.field))
		}
		args = append(args, "%"+sc.keyword+"%")
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

func (s *StructuredSearch) analyzeResults(ctx context.Context, query string, ent *model.Enterprise, policies []model.Policy) (string, []uint, string) {
	// 只分析前 5 条，避免过长
	if len(policies) > 5 {
		policies = policies[:5]
	}
	if len(policies) == 0 {
		userMsg := fmt.Sprintf(
			"企业信息：行业=%s、规模=%s、地址=%s\n用户搜索：%s\n\n数据库中未找到匹配的政策。请分析可能的原因并给出建议。\n严格按照以下 JSON 格式返回，不要附带其他内容：\n{\"text\":\"你的分析内容，200字以内\"}",
			ent.Industry, ent.Scale, ent.Address, query,
		)
		result, err := chatAndParse[analysisResult](s.aiSvc, ctx, "search_analysis", s.aiSvc.prompts.search,
			userMsg, "AI分析失败")
		if err != nil {
			return "", nil, ""
		}
		return result.Text, nil, ""
	}

	var policyBriefs []string
	for _, p := range policies {
		ef := p.ExtractedFields
		summary := ""
		amount := ""
		deadline := ""
		if ef != nil {
			summary = ef.PolicySummary
			if len(ef.Subsidies) > 0 {
				var amts []string
				for _, s := range ef.Subsidies {
					if s.Amount != "" {
						amts = append(amts, s.Amount)
					}
				}
				amount = strings.Join(amts, ",")
			}
		}
		if p.EndDate != "" {
			deadline = p.EndDate
		}
		policyBriefs = append(policyBriefs,
			fmt.Sprintf("[%d]「%s」摘要：%s，补贴%s，截止%s", p.ID, p.Title, summary, amount, deadline))
	}

	userMsg := fmt.Sprintf(
		"企业信息：行业=%s、规模=%s、地址=%s\n用户搜索：%s\n\n以下是数据库中关键词匹配到的相关政策：\n%s\n\n"+
			"请分析这些政策是否真正满足用户需求（尤其是金额、时间等精确条件）。\n"+
			"如果满足，给出个性化的推荐理由和注意事项（包括补贴金额是否匹配、截止时间是否充裕等）；\n"+
			"如果不满足，说明具体原因（如金额超出预算、截止时间太近等）。\n"+
			"全部分析完成后，综合、系统性地评估一下本次检索的效果，以高、一般、低做评价。\n"+
			"严格按照以下 JSON 格式返回，不要附带其他内容：\n"+
			`{"text":"你的分析内容，200字以内","ranked_ids":[最匹配的ID,按推荐度降序],"effect":"high、partial或者low，分别代表高、一般、低，用于评估本次检索的效果"}`,
		ent.Industry, ent.Scale, ent.Address, query,
		strings.Join(policyBriefs, "\n"),
	)

	result, err := chatAndParse[analysisResult](s.aiSvc, ctx, "search_analysis", s.aiSvc.prompts.search,
		userMsg, "AI分析失败")
	if err != nil {
		return "", nil, ""
	}
	return result.Text, result.RankedIDs, result.Effect
}

type analysisResult struct {
	Text      string `json:"text"`
	RankedIDs []uint `json:"ranked_ids"`
	Effect    string `json:"effect"`
}
