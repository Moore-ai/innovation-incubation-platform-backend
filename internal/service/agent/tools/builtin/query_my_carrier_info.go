package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"

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

func (t *QueryMyCarrierInfo) Name() string { return "query_my_carrier_info" }
func (t *QueryMyCarrierInfo) Description() string {
	return "查询当前载体用户的基本信息（名称、类型、地址等）"
}
func (t *QueryMyCarrierInfo) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryMyCarrierInfo) InputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{},
		"required":[]
	}`)
}

func (t *QueryMyCarrierInfo) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"carrier": {"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"},"type":{"type":"string"},"address":{"type":"string"},"area":{"type":"string"},"manager_name":{"type":"string"},"contact_phone":{"type":"string"},"scale":{"type":"string"},"incubation_count":{"type":"integer"},"created_at":{"type":"string"}}}
		},
		"required":["carrier"]
	}`)
}

func (t *QueryMyCarrierInfo) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	carrier, err := t.carrierRepo.FindCarrierByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到用户关联的实体信息，请确认账号已注册")
		}
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"carrier": carrier})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
