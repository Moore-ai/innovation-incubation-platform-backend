package builtin

import (
	"context"
	"encoding/json"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*GenerateReport)(nil)

type GenerateReport struct{}

func NewGenerateReport() *GenerateReport { return &GenerateReport{} }

func (t *GenerateReport) Name() string             { return "generate_report" }
func (t *GenerateReport) Description() string       { return "根据主题生成数据分析报告（Markdown 格式，含图表）。Agent 会自主规划需要的查询和图表。" }
func (t *GenerateReport) AllowedRoles() []string    { return []string{"government"} }

func (t *GenerateReport) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string","description":"报告主题和要求"}},"required":["prompt"]}`)
}

func (t *GenerateReport) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"markdown":{"type":"string"}}}`)
}

func (t *GenerateReport) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	reply := "正在为你生成《" + input.Prompt + "》报告。请等待 Agent 使用 batch_query 和查询工具逐步完成。"
	b, _ := json.Marshal(map[string]string{"ack": reply})
	return json.RawMessage(b), nil
}
