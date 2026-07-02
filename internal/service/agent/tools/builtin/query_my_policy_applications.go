package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryMyPolicyApplications)(nil)

type QueryMyPolicyApplications struct {
	entRepo *repository.EnterpriseRepo
}

func NewQueryMyPolicyApplications(entRepo *repository.EnterpriseRepo) *QueryMyPolicyApplications {
	return &QueryMyPolicyApplications{entRepo: entRepo}
}

func (t *QueryMyPolicyApplications) Name() string          { return "query_my_policy_applications" }
func (t *QueryMyPolicyApplications) Description() string   { return "查询当前企业已审批通过的政策申报记录" }
func (t *QueryMyPolicyApplications) AllowedRoles() []string { return []string{"enterprise"} }

func (t *QueryMyPolicyApplications) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *QueryMyPolicyApplications) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"applications":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}},"required":["applications","total"]}`)
}

func (t *QueryMyPolicyApplications) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	ent, err := t.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, err
	}
	apps, err := t.entRepo.FindApprovedApplications(ent.ID)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"applications": apps, "total": len(apps)})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
