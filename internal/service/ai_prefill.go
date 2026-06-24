package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// PrefillApplication prefills the specified material template with enterprise data.
// Returns prefilled JSON matching the material's FormSchema, or empty map if templateID is 0.
func (s *AIService) PrefillApplication(ctx context.Context, userID uint, policyID uint, templateID uint) (model.JSONMap, error) {
	if templateID == 0 {
		return model.JSONMap{}, nil
	}

	ent, err := s.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}

	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}

	// 查找指定模板 ID 的材料
	if policy.Requirements == nil {
		return nil, errcode.ErrInvalidParams.WithMsg("政策暂无申报要求")
	}
	var targetMaterial model.ApplicationMaterial
	found := false
	for _, m := range policy.Requirements.ApplicationMaterials {
		if m.MaterialTemplate != nil && m.MaterialTemplate.ID == templateID {
			targetMaterial = m
			found = true
			break
		}
	}

	if !found {
		return nil, errcode.ErrInvalidParams.WithMsg("材料模板不存在")
	}
	if targetMaterial.MaterialTemplate.FormSchema == nil {
		return nil, errcode.ErrInvalidParams.WithMsg(fmt.Sprintf("材料「%s」没有可预填充的模板", targetMaterial.Name))
	}

	schemaJSON, err := json.Marshal(targetMaterial.MaterialTemplate.FormSchema)
	if err != nil {
		return nil, errcode.ErrInternal.WithMsg("模板Schema序列化失败")
	}

	history, err := s.entRepo.FindApprovedApplications(ent.ID)
	if err != nil {
		slog.Warn("failed to load approved applications for prefill", "error", err)
	}
	historyJSON := "[]"
	if len(history) > 0 {
		historyJSON = toJSONString(history)
	}

	userMsg := fmt.Sprintf("企业信息：名称=%s、信用代码=%s、行业=%s、规模=%s、地址=%s、法人=%s\n"+
		"历史申报数据(已通过审批): %s\n\n材料「%s」的模板:\n%s\n\n"+
		"请严格按模板的字段结构生成预填充数据，只返回JSON",
		ent.Name, ent.CreditCode, ent.Industry, ent.Scale, ent.Address, ent.LegalPerson,
		historyJSON, targetMaterial.Name, string(schemaJSON))

	result, err := chatAndParse[model.JSONMap](s, ctx, "prefill", s.prompts.prefill, userMsg, "AI预填充结果解析失败")
	if err != nil {
		return nil, err
	}
	return *result, nil
}
