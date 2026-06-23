package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// PrefillApplication generates prefilled form data for an enterprise based on its profile,
// the target policy's form schema, and approved application history.
func (s *AIService) PrefillApplication(ctx context.Context, userID uint, policyID uint) (model.JSONMap, error) {
	ent, err := s.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}

	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}
	formSchema := "{}"
	if policy.Template.FormSchema != nil {
		b, _ := json.Marshal(policy.Template.FormSchema)
		formSchema = string(b)
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
		"历史申报数据(已通过审批): %s\n\n目标表单 Schema: \n%s\n\n请严格按表单Schema的字段结构输出预填充数据，只返回JSON",
		ent.Name, ent.CreditCode, ent.Industry, ent.Scale, ent.Address, ent.LegalPerson,
		historyJSON, formSchema)

	text, err := s.client.Chat(ctx, s.prompts.prefill, userMsg)
	if err != nil {
		return nil, errcode.ErrAIService.WithMsg("AI服务暂不可用，请手动填写")
	}

	var data model.JSONMap
	if err := json.Unmarshal([]byte(cleanLLMOutput(text)), &data); err != nil {
		return nil, errcode.ErrAIService.WithMsg("AI预填充结果解析失败")
	}
	return data, nil
}
