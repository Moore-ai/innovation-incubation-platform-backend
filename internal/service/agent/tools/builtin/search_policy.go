package builtin

import (
	"context"
	"encoding/json"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/service"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*SearchPolicy)(nil)

type SearchPolicy struct {
	search service.PolicySearch
}

func NewSearchPolicy(search service.PolicySearch) *SearchPolicy {
	return &SearchPolicy{search: search}
}

func (t *SearchPolicy) Name() string         { return "search_policy" }
func (t *SearchPolicy) Description() string  { return "根据关键词、行业、企业规模等条件检索匹配的政策，返回政策列表（含标题、摘要、适用条件、补贴详情）" }
func (t *SearchPolicy) AllowedRoles() []string { return []string{"enterprise", "carrier"} }

func (t *SearchPolicy) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词或问题描述"},"industry":{"type":"string","description":"行业"},"scale":{"type":"string","description":"企业规模"},"region":{"type":"string","description":"区域"}},"required":["query"]}`)
}

func (t *SearchPolicy) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"policies":{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"},"title":{"type":"string"},"summary":{"type":"string"},"department":{"type":"string"}},"required":["id","title"]}},"total":{"type":"integer"}},"required":["policies","total"]}`)
}

func (t *SearchPolicy) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		Query    string `json:"query"`
		Industry string `json:"industry"`
		Scale    string `json:"scale"`
		Region   string `json:"region"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	// Build a combined query from available fields
	var parts []string
	if input.Query != "" {
		parts = append(parts, input.Query)
	}
	if input.Industry != "" {
		parts = append(parts, "行业："+input.Industry)
	}
	if input.Scale != "" {
		parts = append(parts, "规模："+input.Scale)
	}
	if input.Region != "" {
		parts = append(parts, "区域："+input.Region)
	}
	query := strings.Join(parts, " ")

	userID := agent.UserIDFromCtx(ctx)
	role := agent.RoleFromCtx(ctx)
	userType := model.UserRole(role)

	resp, err := t.search.Search(ctx, userID, query, userType)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
