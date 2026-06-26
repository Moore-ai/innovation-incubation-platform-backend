package service

import (
	"context"

	"innovation-incubation-platform-backend/internal/model"
)

// SearchResult 搜索结果，包含政策列表和 AI 分析
type SearchResult struct {
	Policies  []model.Policy `json:"policies"`
	Analysis  string         `json:"analysis"`
	RankedIDs []uint         `json:"ranked_ids,omitempty"`
	Effect    string         `json:"effect"`
}

// PolicySearch 政策搜索器 — 可插拔，通过配置切换实现
type PolicySearch interface {
	Search(ctx context.Context, userID uint, query string) (*SearchResult, error)
}

// SearchCriteria AI 从自然语言中提取的结构化查询条件
type SearchCriteria struct {
	ApplicableIndustries  []string `json:"applicable_industries"`
	ApplicableScales      []string `json:"applicable_scales"`
	ApplicableStatus      string   `json:"applicable_status"`
	SubsidyTypes          []string `json:"subsidy_types"`
	ApplicableRegion      string   `json:"applicable_region"`
	SubsidyAmountKeywords []string `json:"subsidy_amount_keywords"` // 金额关键词，如["10万","20万","30万"]
}
