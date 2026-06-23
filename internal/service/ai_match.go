package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"innovation-incubation-platform-backend/internal/model"
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

	userMsg := fmt.Sprintf("企业画像：行业=%s、规模=%s、地址=%s\n政策标题: %s\n政策条件: %s\n提取字段: %s\n\n请按以下格式返回JSON,不要附带其他内容：\n%s",
		ent.Industry, ent.Scale, ent.Address,
		policy.Title,
		toJSONString(policy.Requirements),
		toJSONString(policy.ExtractedFields),
		`{"level":"high|partial|none|unknown","reason":"给出详细的匹配分析理由,必须包含适用条件和补贴额度等信息(你的对话对象是执行本次政策匹配的企业)"}`,
	)

	text, err := s.client.Chat(ctx, s.prompts.match, userMsg)
	if err != nil {
		slog.Warn("LLM match failed, fallback to rule match", "policy_id", policyID, "error", err)
		return fallbackMatch(ent, policy), nil
	}

	var result PolicyMatchResult
	if err := json.Unmarshal([]byte(cleanLLMOutput(text)), &result); err != nil {
		slog.Warn("LLM match parse failed, fallback to rule match", "error", err)
		return fallbackMatch(ent, policy), nil
	}
	return &result, nil
}

func fallbackMatch(ent *model.Enterprise, policy *model.Policy) *PolicyMatchResult {
	level := FieldMatchRule(ent, policy)
	return &PolicyMatchResult{
		Level:  level,
		Reason: "AI暂不可用，当前显示为自动匹配结果",
	}
}
