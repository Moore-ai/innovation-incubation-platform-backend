package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
)

type extractedFields struct {
	PolicyName           string   `json:"policy_name"`           // 政策名称
	ApplicableIndustries []string `json:"applicable_industries"` // 适用行业
	ApplicableScales     []string `json:"applicable_scales"`     // 适用企业规模
	ApplicableStatus     []string `json:"applicable_status"`     // 适用企业状态（如：初创期、成长期）
	SubsidyType          string   `json:"subsidy_type"`          // 补贴类型（如：资金补贴、税收优惠）
	SubsidyAmount        string   `json:"subsidy_amount"`        // 补贴金额
	SubsidyCondition     string   `json:"subsidy_condition"`     // 补贴条件
	ApplicableRegion     string   `json:"applicable_region"`     // 适用区域；也可能是 JSON 数组
	RequiredDocuments    []string `json:"required_documents"`    // 所需材料清单
}

func (s *AIService) ExtractPolicy(ctx context.Context, policy *model.Policy) error {
	userMsg := fmt.Sprintf("政策标题：%s\n政策内容：%s\n\n请严格按以下格式返回JSON,不要附带其他内容：\n%s",
		policy.Title,
		toJSONString(policy.Requirements),
		`{"policy_name":"政策名称","applicable_industries":["适用行业列表"],"applicable_scales":["适用企业规模，如大型、中型、小型、微型"],"applicable_status":"适用企业状态，如：初创期、成长期","subsidy_type":"补贴类型，如：资金补贴、税收优惠","subsidy_amount":"补贴金额","subsidy_condition":"补贴的具体条件","applicable_region":"适用区域，比如安徽省合肥市蜀山区","required_documents":[所需材料清单]}`,
	)
	fields, err := chatAndParse[extractedFields](s, ctx, s.prompts.extract, userMsg, "AI提取结果解析失败")
	if err != nil {
		return err
	}
	b, _ := json.Marshal(fields)
	var extracted model.JSONMap
	json.Unmarshal(b, &extracted)
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
