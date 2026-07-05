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

var _ agenttools.Tool = (*QueryIncubationRecords)(nil)

type QueryIncubationRecords struct {
	entRepo *repository.EnterpriseRepo
}

func NewQueryIncubationRecords(entRepo *repository.EnterpriseRepo) *QueryIncubationRecords {
	return &QueryIncubationRecords{entRepo: entRepo}
}

func (t *QueryIncubationRecords) Name() string { return "query_incubation_records" }
func (t *QueryIncubationRecords) Description() string {
	return "查询当前企业的入驻申请记录，支持分页"
}
func (t *QueryIncubationRecords) AllowedRoles() []string { return []string{"enterprise"} }

func (t *QueryIncubationRecords) InputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"page":{
				"type":"integer",
				"description":"页码，默认1"
			},
			"page_size":{
				"type":"integer",
				"description":"每页条数，默认10"
			}
		},
		"required":[]
	}`)
}

func (t *QueryIncubationRecords) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"records":{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"},"enterprise_id":{"type":"integer"},"carrier_id":{"type":"integer"},"incubate_status":{"type":"string"},"incubate_start":{"type":"string"},"incubate_end":{"type":"string"},"status":{"type":"string"},"created_at":{"type":"string"}}}}},
			"total":{
				"type":"integer"
			}
		},
		"required":["records","total"]
	}`)
}

func (t *QueryIncubationRecords) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
	ent, err := t.entRepo.FindEnterpriseByUserID(userID)
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
	records, total, err := t.entRepo.ListIncubationByEnterprise(ent.ID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"records": records, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
