package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryEnterpriseInfo)(nil)

type QueryEnterpriseInfo struct {
	entRepo *repository.EnterpriseRepo
}

func NewQueryEnterpriseInfo(entRepo *repository.EnterpriseRepo) *QueryEnterpriseInfo {
	return &QueryEnterpriseInfo{entRepo: entRepo}
}

func (t *QueryEnterpriseInfo) Name() string          { return "query_enterprise_info" }
func (t *QueryEnterpriseInfo) Description() string   { return "查询当前企业的入驻信息、入驻状态、申报进度等。仅企业用户可用" }
func (t *QueryEnterpriseInfo) AllowedRoles() []string { return []string{"enterprise"} }

func (t *QueryEnterpriseInfo) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *QueryEnterpriseInfo) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"enterprise":{"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"},"industry":{"type":"string"},"scale":{"type":"string"}},"required":["id","name"]}},"required":["enterprise"]}`)
}

func (t *QueryEnterpriseInfo) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	ent, err := t.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, err
	}
	resp := map[string]any{
		"enterprise": map[string]any{
			"id":       ent.ID,
			"name":     ent.Name,
			"industry": ent.Industry,
			"scale":    ent.Scale,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
