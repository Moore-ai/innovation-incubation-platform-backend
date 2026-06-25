package service

import (
	"context"
	"innovation-incubation-platform-backend/internal/model"
)

// PolicySearch 政策搜索器 — 可插拔，通过配置切换实现
type PolicySearch interface {
	Search(ctx context.Context, userID uint, query string) ([]model.Policy, error)
}

// SearchCriteria AI 从自然语言中提取的结构化查询条件
type SearchCriteria struct {
	ApplicableIndustries []string `json:"applicable_industries"`
	ApplicableScales     []string `json:"applicable_scales"`
	ApplicableStatus     string   `json:"applicable_status"`
	SubsidyTypes         []string `json:"subsidy_types"`
	ApplicableRegion     string   `json:"applicable_region"`
}
