package service

import (
	"context"
	"fmt"

	"innovation-incubation-platform-backend/pkg/errcode"
)

// PolicyMatchResult represents the LLM-based matching outcome for a policy.
type PolicyMatchResult struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

// MatchPolicy performs LLM-based policy matching for an enterprise against a specific policy.
// Falls back to rule-based matching on LLM failure.
func (s *AIService) MatchPolicy(ctx context.Context, userID uint, policyID uint) (*PolicyMatchResult, error) {
	ent, err := s.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}

	var extractedFields any
	if policy.ExtractedFields != nil {
		extractedFields = policy.ExtractedFields
	} else {
		extractedFields = policy.Requirements
	}
	userMsg := fmt.Sprintf("企业画像：行业=%s、规模=%s、地址=%s\n政策标题: %s\n政策条件: %s\n提取字段: %s\n\n请按以下格式返回JSON,不要附带其他内容：\n%s",
		ent.Industry, ent.Scale, ent.Address,
		policy.Title,
		toJSONString(policy.Requirements),
		toJSONString(extractedFields),
		`{"level":"high|partial|none|unknown","reason":"给出详细的匹配分析理由,必须包含适用条件和补贴额度等信息(你的对话对象是执行本次政策匹配的企业)"}`,
	)

	result, err := chatAndParse[PolicyMatchResult](s, ctx, "match", s.prompts.match, userMsg, "AI匹配失败")
	if err != nil {
		return fallbackMatch(), nil
	}
	return result, nil
}

func fallbackMatch() *PolicyMatchResult {
	return &PolicyMatchResult{
		Level:  "unknown",
		Reason: "AI暂不可用",
	}
}
