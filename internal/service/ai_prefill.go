package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

func (s *AIService) compilePrefillChain(ctx context.Context) (compose.Runnable[map[string]any, model.JSONMap], error) {
	// prep: 在 Chain 内将 ent 对象序列化为模板变量
	prep := compose.InvokableLambda(func(_ context.Context, input map[string]any) (map[string]any, error) {
		ent, ok := input["enterprise"].(*model.Enterprise)
		if !ok {
			return nil, fmt.Errorf("prep: missing or invalid enterprise")
		}
		return map[string]any{
			"name":         ent.Name,
			"credit_code":  ent.CreditCode,
			"industry":     ent.Industry,
			"scale":        ent.Scale,
			"address":      ent.Address,
			"legal_person": ent.LegalPerson,
			"history":      input["history"],
			"form_schema":  input["form_schema"],
		}, nil
	})

	tmpl := prompt.FromMessages(schema.FString,
		schema.SystemMessage(s.prompts.prefill),
		schema.UserMessage("企业信息：名称={name}、信用代码={credit_code}、行业={industry}、规模={scale}、地址={address}、法人={legal_person}\n"+
			"历史申报数据(已通过审批): {history}\n\n"+
			"目标表单 Schema: \n{form_schema}\n\n"+
			"请严格按表单 Schema 的字段结构输出预填充数据，只返回 JSON"),
	)

	chain := compose.NewChain[map[string]any, model.JSONMap]()
	chain.AppendLambda(prep)
	chain.AppendChatTemplate(tmpl)
	chain.AppendChatModel(s.cm)
	chain.AppendLambda(compose.InvokableLambda(func(_ context.Context, msg *schema.Message) (model.JSONMap, error) {
		var data model.JSONMap
		if err := json.Unmarshal([]byte(msg.Content), &data); err != nil {
			return nil, err
		}
		return data, nil
	}))
	return chain.Compile(ctx)
}

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

	chain, err := s.compilePrefillChain(ctx)
	if err != nil {
		return nil, errcode.ErrAIService.WithMsg("AI服务暂不可用，请手动填写")
	}

	return chain.Invoke(ctx, map[string]any{
		"enterprise":  ent,
		"history":     historyJSON,
		"form_schema": formSchema,
	})
}
