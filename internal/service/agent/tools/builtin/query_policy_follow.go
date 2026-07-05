package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryPolicyFollow)(nil)

type QueryPolicyFollow struct {
	followRepo *repository.PolicyFollowRepo
}

func NewQueryPolicyFollow(followRepo *repository.PolicyFollowRepo) *QueryPolicyFollow {
	return &QueryPolicyFollow{followRepo: followRepo}
}

func (t *QueryPolicyFollow) Name() string           { return "query_policy_follow" }
func (t *QueryPolicyFollow) Description() string    { return "查询当前用户关注的政策列表" }
func (t *QueryPolicyFollow) AllowedRoles() []string { return []string{"enterprise", "carrier"} }

func (t *QueryPolicyFollow) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"page":{"type":"integer","description":"页码，默认1"},"page_size":{"type":"integer","description":"每页条数，默认10"}},"required":[]}`)
}

func (t *QueryPolicyFollow) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"follows":{
				"type":"array",
				"items":{
					"type":"object",
					"properties":{
						"id":{
							"type":"integer"
						},
						"policy":{
							"type":"object",
							"properties":{
								"id":{
									"type":"integer"
								},
								"title":{
									"type":"string"
								},
								"department":{
									"type":"string"
								}
							},
							"required":["id","title"]
						}
					},
					"required":["id","policy"]
				}
			},
			"total":{
				"type":"integer"
			}
		},
		"required":["follows","total"]
	}`)
}

func (t *QueryPolicyFollow) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
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
	follows, total, err := t.followRepo.ListByEnterprise(userID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"follows": follows, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
