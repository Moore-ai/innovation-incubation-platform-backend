package service

import (
	"context"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// SearchResult 搜索结果，包含政策列表和 AI 分析（Policies 按推荐度降序排列）
type SearchResult struct {
	Policies []model.Policy `json:"policies"`
	Analysis string         `json:"analysis"`
	Found    bool           `json:"found"`
	Effect   string         `json:"effect"`
}

// PolicySearch 政策搜索器 — 可插拔，通过配置切换实现
type PolicySearch interface {
	Search(ctx context.Context, userID uint, query string, userType model.UserRole) (*SearchResult, error)
}

// SearchCriteria AI 从自然语言中提取的结构化查询条件
type SearchCriteria struct {
	ApplicableIndustries  []string `json:"applicable_industries"`
	ApplicableScales      []string `json:"applicable_scales"`
	ApplicableStatus      string   `json:"applicable_status"`
	SubsidyTypes          []string `json:"subsidy_types"`
	ApplicableRegion      string   `json:"applicable_region"`
	SubsidyAmountKeywords []string `json:"subsidy_amount_keywords"`
}

// buildSearchProfile 根据用户角色查询企业或载体画像，返回格式化后的 profile 字符串。
func buildSearchProfile(userID uint, userType model.UserRole, carrierRepo *repository.CarrierRepo, entRepo *repository.EnterpriseRepo) (string, error) {
	if !userType.IsValid() {
		return "", errcode.ErrInvalidParams.WithMsg("无效的用户类型")
	}
	switch userType {
	case model.UserRoleCarrier:
		if carrierRepo == nil {
			return "", errcode.ErrInternal.WithMsg("搜索服务配置错误")
		}
		carrier, err := carrierRepo.FindCarrierByUserID(userID)
		if err != nil {
			return "", errcode.ErrForbidden.WithMsg("载体信息未找到")
		}
		return carrierProfileStr(carrier), nil
	default:
		if entRepo == nil {
			return "", errcode.ErrInternal.WithMsg("搜索服务配置错误")
		}
		ent, err := entRepo.FindEnterpriseByUserID(userID)
		if err != nil {
			return "", errcode.ErrForbidden.WithMsg("无权访问")
		}
		return enterpriseProfileStr(ent), nil
	}
}
