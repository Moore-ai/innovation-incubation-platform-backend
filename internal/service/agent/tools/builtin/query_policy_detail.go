package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryPolicyDetail)(nil)

type QueryPolicyDetail struct {
	govRepo *repository.GovernmentRepo
}

func NewQueryPolicyDetail(govRepo *repository.GovernmentRepo) *QueryPolicyDetail {
	return &QueryPolicyDetail{govRepo: govRepo}
}

func (t *QueryPolicyDetail) Name() string { return "query_policy_detail" }
func (t *QueryPolicyDetail) Description() string {
	return "根据政策ID获取政策完整信息（含申报条件、补贴详情、所需材料、办理流程）"
}
func (t *QueryPolicyDetail) AllowedRoles() []string { return []string{"enterprise", "carrier", "government"} }

func (t *QueryPolicyDetail) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"policy_id":{"type":"integer","description":"政策ID"}},"required":["policy_id"]}`)
}

func (t *QueryPolicyDetail) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"policy": {"type":"object","properties":{"id":{"type":"integer"},"title":{"type":"string"},"department":{"type":"string"},"target_role":{"type":"string"},"status":{"type":"string"},"start_date":{"type":"string"},"end_date":{"type":"string"},"published_at":{"type":"string"},"requirements":{"type":"object"},"extracted_fields":{"type":"object"}}}
		},
		"required":["policy"]
	}`)
}

func (t *QueryPolicyDetail) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		PolicyID uint `json:"policy_id"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	policy, err := t.govRepo.FindPolicyByID(input.PolicyID)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"policy": policy})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
