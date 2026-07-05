package builtin

import (
	"context"
	"encoding/json"

	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*QueryMyFiles)(nil)

type QueryMyFiles struct {
	fileRepo *repository.FileRepo
}

func NewQueryMyFiles(fileRepo *repository.FileRepo) *QueryMyFiles {
	return &QueryMyFiles{fileRepo: fileRepo}
}

func (t *QueryMyFiles) Name() string { return "query_my_files" }
func (t *QueryMyFiles) Description() string {
	return "查询当前用户上传的文件列表，支持分页"
}
func (t *QueryMyFiles) AllowedRoles() []string { return []string{"enterprise", "carrier"} }

func (t *QueryMyFiles) InputSchema() json.RawMessage {
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

func (t *QueryMyFiles) OutputSchema() json.RawMessage {
	return json.RawMessage(`
	{
		"type":"object",
		"properties":{
			"files":{"type":"array","items":{"type":"object","properties":{"id":{"type":"integer"},"filename":{"type":"string"},"mime_type":{"type":"string"},"size":{"type":"integer"},"summary":{"type":"string"},"created_at":{"type":"string"}}}}},
			"total":{
				"type":"integer"
			}
		},
		"required":["files","total"]
	}`)
}

func (t *QueryMyFiles) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
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
	files, total, err := t.fileRepo.ListByUploader(userID, input.Page, input.PageSize)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(map[string]any{"files": files, "total": total})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
