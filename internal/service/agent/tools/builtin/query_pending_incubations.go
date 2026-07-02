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

var _ agenttools.Tool = (*QueryPendingIncubations)(nil)

type QueryPendingIncubations struct {
	carrierRepo *repository.CarrierRepo
}

func NewQueryPendingIncubations(carrierRepo *repository.CarrierRepo) *QueryPendingIncubations {
	return &QueryPendingIncubations{carrierRepo: carrierRepo}
}

func (t *QueryPendingIncubations) Name() string          { return "query_pending_incubations" }
func (t *QueryPendingIncubations) Description() string   { return "查询待审核的企业入驻申请列表，支持分页" }
func (t *QueryPendingIncubations) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryPendingIncubations) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"page":{"type":"integer","description":"页码，默认1"},"page_size":{"type":"integer","description":"每页条数，默认10"}},"required":[]}`)
}

func (t *QueryPendingIncubations) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"records":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}},"required":["records","total"]}`)
}

func (t *QueryPendingIncubations) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	carrier, err := t.carrierRepo.FindCarrierByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到用户关联的实体信息，请确认账号已注册")
		}
		return nil, err
	}
	var input struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	if input.Page <= 0 {
		input.Page = 1
	}
	if input.PageSize <= 0 {
		input.PageSize = 10
	}
	records, total, err := t.carrierRepo.ListPendingIncubations(carrier.ID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"records": records, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
