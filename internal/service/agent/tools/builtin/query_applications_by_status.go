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

var _ agenttools.Tool = (*QueryApplicationsByStatus)(nil)

type QueryApplicationsByStatus struct {
	carrierRepo *repository.CarrierRepo
}

func NewQueryApplicationsByStatus(carrierRepo *repository.CarrierRepo) *QueryApplicationsByStatus {
	return &QueryApplicationsByStatus{carrierRepo: carrierRepo}
}

func (t *QueryApplicationsByStatus) Name() string { return "query_applications_by_status" }
func (t *QueryApplicationsByStatus) Description() string {
	return "根据审核状态（pending、approved、rejected）查询企业政策申报记录，支持分页"
}
func (t *QueryApplicationsByStatus) AllowedRoles() []string { return []string{"carrier"} }

func (t *QueryApplicationsByStatus) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","description":"审核状态：pending、approved、rejected"},"page":{"type":"integer"},"page_size":{"type":"integer"}},"required":["status"]}`)
}

func (t *QueryApplicationsByStatus) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"applications":{"type":"array","items":{"type":"object"}},"total":{"type":"integer"}},"required":["applications","total"]}`)
}

func (t *QueryApplicationsByStatus) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	carrier, err := t.carrierRepo.FindCarrierByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到用户关联的实体信息，请确认账号已注册")
		}
		return nil, err
	}
	var input struct {
		Status   string `json:"status"`
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
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
	apps, total, err := t.carrierRepo.ListEnterpriseApplicationsByStatus(carrier.ID, input.Status, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"applications": apps, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
