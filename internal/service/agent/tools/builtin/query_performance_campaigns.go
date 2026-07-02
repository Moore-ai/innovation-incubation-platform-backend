package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
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
