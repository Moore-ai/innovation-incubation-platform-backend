package builtin

import (
	"context"
	"encoding/json"
	"time"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryAppeal)(nil)

type QueryAppeal struct {
	appealRepo *repository.AppealRepo
}

func NewQueryAppeal(appealRepo *repository.AppealRepo) *QueryAppeal {
	return &QueryAppeal{appealRepo: appealRepo}
}

func (t *QueryAppeal) Name() string { return "query_appeal" }
func (t *QueryAppeal) Description() string {
	return "查询当前用户提交的诉求（反馈/建议）的处理状态和结果"
}
func (t *QueryAppeal) AllowedRoles() []string { return []string{"enterprise", "carrier"} }
func (t *QueryAppeal) Timeout() time.Duration { return agenttools.DefaultTimeout() }

func (t *QueryAppeal) InputSchema() json.RawMessage {
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

func (t *QueryAppeal) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"appeals":{
				"type":"array",
				"items":{
					"type":"object",
					"properties":{
						"id":{
							"type":"integer"
						},
						"problem_type":{
							"type":"string"
						},
						"content":{
							"type":"string"
						},
						"status":{
							"type":"string"
						}
					},
					"required":["id","status"]
				}
			},
			"total":{
				"type":"integer"
			}
		},
		"required":["appeals","total"]
	}`)
}

func (t *QueryAppeal) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	userID := agent.UserIDFromCtx(ctx)
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
	appeals, total, err := t.appealRepo.ListBySubmitter(userID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"appeals": appeals, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
