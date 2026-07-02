package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryMyCarrierInfo)(nil)

type QueryMyCarrierInfo struct {
	carrierRepo *repository.CarrierRepo
}

func NewQueryMyCarrierInfo(carrierRepo *repository.CarrierRepo) *QueryMyCarrierInfo {
	return &QueryMyCarrierInfo{carrierRepo: carrierRepo}
}

func (t *QueryMyCarrierInfo) Name() string          { return "query_my_carrier_info" }
func (t *QueryMyCarrierInfo) Description() string   { return "查询当前载体用户的基本信息（名称、类型、地址等）" }
func (t *QueryMyCarrierInfo) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryMyCarrierInfo) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}

func (t *QueryMyCarrierInfo) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"carrier":{"type":"object"}},"required":["carrier"]}`)
}

func (t *QueryMyCarrierInfo) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	carrier, err := t.carrierRepo.FindCarrierByUserID(userID)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"carrier": carrier})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
