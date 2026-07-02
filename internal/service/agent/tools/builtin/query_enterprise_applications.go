package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryEnterpriseApplications)(nil)

type QueryEnterpriseApplications struct {
	carrierRepo *repository.CarrierRepo
}

func NewQueryEnterpriseApplications(carrierRepo *repository.CarrierRepo) *QueryEnterpriseApplications {
	return &QueryEnterpriseApplications{carrierRepo: carrierRepo}
}

func (t *QueryEnterpriseApplications) Name() string          { return "query_enterprise_applications" }
func (t *QueryEnterpriseApplications) Description() string   { return "查询企业提交的政策申报记录，支持分页" }
func (t *QueryEnterpriseApplications) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryEnterpriseApplications) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"page":{"type":"integer","description":"页码，默认1"},"page_size":{"type":"integer","description":"每页条数，默认10"}},"required":[]}`)
}

func (t *QueryEnterpriseApplications) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"applications":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}},"required":["applications","total"]}`)
}

func (t *QueryEnterpriseApplications) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	carrier, err := t.carrierRepo.FindCarrierByUserID(userID)
	if err != nil {
		return nil, err
	}
	var input struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	json.Unmarshal(args, &input)
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 10
	}
	apps, total, err := t.carrierRepo.ListEnterpriseApplicationsForCarrier(carrier.ID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"applications": apps, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
