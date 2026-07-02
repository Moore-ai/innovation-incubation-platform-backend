package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryPendingChanges)(nil)

type QueryPendingChanges struct {
	carrierRepo *repository.CarrierRepo
}

func NewQueryPendingChanges(carrierRepo *repository.CarrierRepo) *QueryPendingChanges {
	return &QueryPendingChanges{carrierRepo: carrierRepo}
}

func (t *QueryPendingChanges) Name() string          { return "query_pending_changes" }
func (t *QueryPendingChanges) Description() string   { return "查询待审核的企业变更申请列表，支持分页" }
func (t *QueryPendingChanges) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryPendingChanges) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"page":{"type":"integer","description":"页码，默认1"},"page_size":{"type":"integer","description":"每页条数，默认10"}},"required":[]}`)
}

func (t *QueryPendingChanges) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"changes":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}},"required":["changes","total"]}`)
}

func (t *QueryPendingChanges) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
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
	changes, total, err := t.carrierRepo.ListPendingChanges(carrier.ID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"changes": changes, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
