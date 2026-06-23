package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
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

func (s *AIService) ExtractPolicy(ctx context.Context, policy *model.Policy) error {
	userMsg := fmt.Sprintf("政策标题：%s\n政策内容：%s\n\n请严格按以下格式返回JSON,不要附带其他内容：\n%s",
		policy.Title,
		toJSONString(policy.Requirements),
		`{"policy_name":"","applicable_industries":[],"applicable_scales":[],"applicable_status":[],"subsidy_type":"","subsidy_amount":"","subsidy_condition":"","applicable_region":"","required_documents":[]}`,
	)
	text, err := s.client.Chat(ctx, s.prompts.extract, userMsg)
	if err != nil {
		return errcode.ErrAIService.WithMsg("AI提取服务暂不可用")
	}
	var fields extractedFields
	if err := json.Unmarshal([]byte(cleanLLMOutput(text)), &fields); err != nil {
		slog.Error("AI extract: failed to parse", "error", err)
		return errcode.ErrAIService.WithMsg("AI提取结果解析失败")
	}
	b, _ := json.Marshal(fields)
	var extracted model.JSONMap
	if err := json.Unmarshal(b, &extracted); err != nil {
		slog.Error("AI extract: failed to convert extracted fields", "error", err)
		return errcode.ErrAIService.WithMsg("AI提取结果解析失败")
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

func toJSONString(v any) string {
	if v == nil {
		return "{}"
	}
	b, _ := json.Marshal(v)
	if len(b) == 0 || string(b) == "null" {
		return "{}"
	}
	return string(b)
}
