package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/prompt"
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
		}, nil
	})

	tmpl := prompt.FromMessages(schema.FString,
		schema.SystemMessage(s.prompts.prefill),
		schema.UserMessage("企业信息：名称={name}、信用代码={credit_code}、行业={industry}、规模={scale}、地址={address}、法人={legal_person}\n历史申报数据（已通过审批）：{history}"),
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

// PrefillApplication generates prefilled form data for an enterprise based on its profile
// and approved application history. Falls back gracefully on AI failure.
func (s *AIService) PrefillApplication(ctx context.Context, userID uint) (model.JSONMap, error) {
	ent, err := s.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
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
		"enterprise": ent,
		"history":    historyJSON,
	})
}
