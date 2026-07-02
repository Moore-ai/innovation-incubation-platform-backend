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

var _ agenttools.Tool = (*QueryPerformanceCampaigns)(nil)

type QueryPerformanceCampaigns struct {
	carrierRepo *repository.CarrierRepo
}

func NewQueryPerformanceCampaigns(carrierRepo *repository.CarrierRepo) *QueryPerformanceCampaigns {
	return &QueryPerformanceCampaigns{carrierRepo: carrierRepo}
}

func (t *QueryPerformanceCampaigns) Name() string          { return "query_performance_campaigns" }
func (t *QueryPerformanceCampaigns) Description() string   { return "查询当前可参与的绩效评估活动列表，支持分页" }
func (t *QueryPerformanceCampaigns) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryPerformanceCampaigns) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"page":{"type":"integer","description":"页码，默认1"},"page_size":{"type":"integer","description":"每页条数，默认10"}},"required":[]}`)
}

func (t *QueryPerformanceCampaigns) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"campaigns":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}},"required":["campaigns","total"]}`)
}

func (t *QueryPerformanceCampaigns) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	carrier, err := t.carrierRepo.FindCarrierByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到用户关联的实体信息，请确认账号已注册")
		}
		return nil, err
	}
	_ = carrier // campaigns are global but identity check is done
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
	campaigns, total, err := t.carrierRepo.ListActiveCampaigns(input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"campaigns": campaigns, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
