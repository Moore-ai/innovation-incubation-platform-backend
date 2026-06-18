package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"innovation-incubation-platform-backend/internal/model"
)

type extractedFields struct {
	PolicyName           string   `json:"policy_name"`
	ApplicableIndustries []string `json:"applicable_industries"`
	ApplicableScales     []string `json:"applicable_scales"`
	ApplicableStatus     []string `json:"applicable_status"`
	SubsidyType          string   `json:"subsidy_type"`
	SubsidyAmount        string   `json:"subsidy_amount"`
	SubsidyCondition     string   `json:"subsidy_condition"`
	ApplicableRegion     string   `json:"applicable_region"` // could also be a JSON array - handled by FieldMatchRule
	RequiredDocuments    []string `json:"required_documents"`
}

func (s *AIService) compileExtractChain(ctx context.Context) (compose.Runnable[map[string]any, *extractedFields], error) {
	tmpl := prompt.FromMessages(schema.FString,
		schema.SystemMessage(s.prompts.extract),
		schema.UserMessage("政策标题：{title}\n政策内容: {content}\n\n"+
			"请严格按以下格式返回 JSON, 不要附带其他内容：\n{output_schema}"),
	)

	chain := compose.NewChain[map[string]any, *extractedFields]()
	chain.AppendChatTemplate(tmpl)
	chain.AppendChatModel(s.cm)
	chain.AppendLambda(compose.InvokableLambda(func(_ context.Context, msg *schema.Message) (*extractedFields, error) {
		var fields extractedFields
		err := json.Unmarshal([]byte(cleanLLMOutput(msg.Content)), &fields)
		return &fields, err
	}))
	return chain.Compile(ctx)
}

func (s *AIService) ExtractPolicy(ctx context.Context, policyID uint) error {
	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return err
	}

	chain, err := s.compileExtractChain(ctx)
	if err != nil {
		return err
	}

	fields, err := chain.Invoke(ctx, map[string]any{
		"title":         policy.Title,
		"content":       toJSONString(policy.Conditions),
		"output_schema": `{"policy_name":"","applicable_industries":[],"applicable_scales":[],"applicable_status":[],"subsidy_type":"","subsidy_amount":"","subsidy_condition":"","applicable_region":"","required_documents":[]}`,
	})
	if err != nil {
		return err
	}

	b, _ := json.Marshal(fields)
	var extracted model.JSONMap
	if err := json.Unmarshal(b, &extracted); err != nil {
		slog.Error("AI extract: failed to unmarshal extracted fields", "error", err)
		return err
	}
	policy.ExtractedFields = extracted
	return s.govRepo.UpdatePolicy(policy)
}

// cleanLLMOutput strips markdown code block wrapping and extra whitespace from LLM JSON output.
func cleanLLMOutput(s string) string {
	cleaned := strings.TrimSpace(s)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

func toJSONString(v interface{}) string {
	b, _ := json.Marshal(v)
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}
