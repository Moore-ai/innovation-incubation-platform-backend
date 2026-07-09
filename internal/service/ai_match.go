package service

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// PolicyMatchResult represents the LLM-based matching outcome for a policy.
type PolicyMatchResult struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

type policyMatchListResult struct {
	Matches []policyMatchListItem `json:"matches"`
}

type policyMatchListItem struct {
	PolicyID uint   `json:"policy_id"`
	Level    string `json:"level"`
	Reason   string `json:"reason"`
}

// MatchPolicy performs LLM-based policy matching for an enterprise against a specific policy.
func (s *AIService) MatchPolicy(ctx context.Context, userID uint, policyID uint) (*PolicyMatchResult, error) {
	ent, err := s.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}

	result, err := s.matchOnePolicy(ctx, ent, *policy)
	if err != nil {
		return fallbackMatch(), nil
	}
	return result, nil
}

func (s *AIService) MatchPolicyListForEnterprise(ctx context.Context, ent *model.Enterprise, policies []model.Policy) map[uint]PolicyMatchResult {
	results := make(map[uint]PolicyMatchResult, len(policies))
	if ent == nil || len(policies) == 0 {
		return results
	}
	briefs := make([]string, 0, len(policies))
	for _, policy := range policies {
		briefs = append(briefs, buildPolicyMatchBrief(policy))
	}
	userMsg := fmt.Sprintf("%s\n\n以下是待匹配政策：\n%s\n\n请只根据企业所属领域和企业简介判断每个政策的匹配度；如果企业简介为空，只根据所属领域判断。严格返回 JSON，不要附带其他内容：\n%s",
		enterprisePolicyMatchProfile(ent),
		strings.Join(briefs, "\n---\n"),
		`{"matches":[{"policy_id":1,"level":"high|partial|none|unknown","reason":"简短说明匹配依据"}]}`,
	)
	result, err := ChatAndParse[policyMatchListResult](s, ctx, "match_list", s.prompts.match, userMsg, "AI匹配失败")
	if err != nil {
		for _, policy := range policies {
			results[policy.ID] = *fallbackMatch()
		}
		return results
	}
	for _, item := range result.Matches {
		results[item.PolicyID] = PolicyMatchResult{Level: normalizeMatchLevel(item.Level), Reason: item.Reason}
	}
	for _, policy := range policies {
		if _, ok := results[policy.ID]; !ok {
			results[policy.ID] = *fallbackMatch()
		}
	}
	return results
}

func (s *AIService) matchOnePolicy(ctx context.Context, ent *model.Enterprise, policy model.Policy) (*PolicyMatchResult, error) {
	userMsg := fmt.Sprintf("%s\n%s\n\n请只根据企业所属领域和企业简介判断政策匹配度；如果企业简介为空，只根据所属领域判断。严格返回 JSON，不要附带其他内容：\n%s",
		enterprisePolicyMatchProfile(ent),
		buildPolicyMatchBrief(policy),
		`{"level":"high|partial|none|unknown","reason":"说明所属领域、企业简介与政策适用对象/支持方向的匹配依据"}`,
	)
	result, err := ChatAndParse[PolicyMatchResult](s, ctx, "match", s.prompts.match, userMsg, "AI匹配失败")
	if err != nil {
		return nil, err
	}
	result.Level = normalizeMatchLevel(result.Level)
	return result, nil
}

func buildPolicyMatchBrief(policy model.Policy) string {
	extractedFields := any(policy.Requirements)
	if policy.ExtractedFields != nil {
		extractedFields = policy.ExtractedFields
	}
	return fmt.Sprintf("policy_id=%d\n政策标题=%s\n政策条件=%s\n提取字段=%s",
		policy.ID, policy.Title, toJSONString(policy.Requirements), toJSONString(extractedFields))
}

func enterprisePolicyMatchProfile(ent *model.Enterprise) string {
	industry := strings.TrimSpace(ent.Industry)
	description := strings.TrimSpace(ent.Description)
	if description == "" {
		return fmt.Sprintf("企业画像：所属领域=%s；企业简介未填写，仅根据所属领域进行政策匹配。", industry)
	}
	return fmt.Sprintf("企业画像：所属领域=%s；企业简介=%s。", industry, description)
}

func normalizeMatchLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high", "partial", "none", "unknown":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "unknown"
	}
}

func fallbackMatch() *PolicyMatchResult {
	return &PolicyMatchResult{
		Level:  "unknown",
		Reason: "AI暂不可用",
	}
}
