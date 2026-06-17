package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// PolicyMatchResult represents the LLM-based matching outcome for a policy.
type PolicyMatchResult struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

// matchParser implements schema.MessageParser[*PolicyMatchResult] by JSON-unmarshaling the message content.
type matchParser struct{}

func (p *matchParser) Parse(_ context.Context, msg *schema.Message) (*PolicyMatchResult, error) {
	var result PolicyMatchResult
	if err := json.Unmarshal([]byte(msg.Content), &result); err != nil {
		return &PolicyMatchResult{Level: "partial", Reason: "AI分析结果格式异常，当前显示为自动匹配结果"}, nil
	}
	return &result, nil
}

func (s *AIService) compileMatchGraph(ctx context.Context) (compose.Runnable[map[string]any, *PolicyMatchResult], error) {
	tmpl := prompt.FromMessages(schema.FString,
		schema.SystemMessage(s.prompts.match),
		schema.UserMessage("企业画像：行业={industry}、规模={scale}、地址={address}\n政策标题：{title}\n政策条件：{conditions}\n提取字段：{extracted}"),
	)

	graph := compose.NewGraph[map[string]any, *PolicyMatchResult]()
	graph.AddChatTemplateNode("prompt", tmpl)
	graph.AddChatModelNode("model", s.cm)
	graph.AddLambdaNode("parse", compose.MessageParser[*PolicyMatchResult](&matchParser{}))
	graph.AddEdge(compose.START, "prompt")
	graph.AddEdge("prompt", "model")
	graph.AddEdge("model", "parse")
	graph.AddEdge("parse", compose.END)
	return graph.Compile(ctx)
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

	graph, err := s.compileMatchGraph(ctx)
	if err != nil {
		slog.Warn("MatchPolicy: compileMatchGraph failed, fallback to rule match", "error", err)
		return fallbackMatch(ent, policy), nil
	}

	extracted := policy.ExtractedFields
	if extracted == nil {
		extracted = policy.Conditions
	}

	result, err := graph.Invoke(ctx, map[string]any{
		"industry":   ent.Industry,
		"scale":      ent.Scale,
		"address":    ent.Address,
		"title":      policy.Title,
		"conditions": toJSONString(policy.Conditions),
		"extracted":  toJSONString(extracted),
	})
	if err != nil {
		slog.Warn("LLM match failed, fallback to rule match", "policy_id", policyID, "error", err)
		return fallbackMatch(ent, policy), nil
	}
	return result, nil
}

func fallbackMatch(ent *model.Enterprise, policy *model.Policy) *PolicyMatchResult {
	level := FieldMatchRule(ent, policy)
	return &PolicyMatchResult{
		Level:  level,
		Reason: "AI暂不可用，当前显示为自动匹配结果",
	}
}
