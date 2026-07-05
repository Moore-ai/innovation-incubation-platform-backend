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

var _ agenttools.Tool = (*QueryChangeHistory)(nil)

type QueryChangeHistory struct {
	entRepo *repository.EnterpriseRepo
}

func NewQueryChangeHistory(entRepo *repository.EnterpriseRepo) *QueryChangeHistory {
	return &QueryChangeHistory{entRepo: entRepo}
}

func (t *QueryChangeHistory) Name() string { return "query_change_history" }
func (t *QueryChangeHistory) Description() string {
	return "查询当前企业的变更记录，支持分页"
}
func (t *QueryChangeHistory) AllowedRoles() []string { return []string{"enterprise"} }

func (t *QueryChangeHistory) InputSchema() json.RawMessage {
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
		},"required":[]
	}`)
}

func (t *QueryChangeHistory) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"changes":{
				"type":"array",
				"items":{
					"type":"object"
				}
			},
			"total":{
				"type":"integer"
			}
		},
		"required":["changes","total"]
	}`)
}

func (t *QueryChangeHistory) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
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
	changes, total, err := t.entRepo.ListChangesByEnterprise(ent.ID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"changes": changes, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
