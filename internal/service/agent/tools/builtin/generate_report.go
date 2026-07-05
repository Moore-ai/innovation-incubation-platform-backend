package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"gorm.io/gorm"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/mcpchart"

	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*GenerateReport)(nil)

type GenerateReport struct {
	ai      *aiclient.Client
	db      *gorm.DB
	engine  *QueryEngine
	configs []*TableConfig
}

func NewGenerateReport(ai *aiclient.Client, db *gorm.DB) *GenerateReport {
	return &GenerateReport{ai: ai, db: db, engine: &QueryEngine{}, configs: TableConfigs}
}

func (t *GenerateReport) Name() string           { return "generate_report" }
func (t *GenerateReport) AllowedRoles() []string { return []string{"government"} }
func (t *GenerateReport) Description() string {
	return "根据政务要求，生成数据分析报告（Markdown 格式，含图表）。"
}

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

	pw := agent.ProgressWriterFromCtx(ctx)

	// Phase 1: Analyst
	sendProgress(pw, "report_start", map[string]any{"phase": "analyst"})
	specs, err := t.runAnalyst(ctx, input.Prompt)
	if err != nil {
		return nil, fmt.Errorf("分析阶段失败: %w", err)
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("分析师未规划任何图表，请细化需求后重试")
	}

	// Phase 2: Executor (并发)
	charts, err := t.runExecutor(ctx, specs, pw)
	if err != nil {
		return nil, fmt.Errorf("执行阶段失败: %w", err)
	}

	// Phase 3: Summarizer
	sendProgress(pw, "report_progress", map[string]any{"phase": "summarizer"})
	markdown, err := t.runSummarizer(ctx, input.Prompt, charts)
	if err != nil {
		return nil, fmt.Errorf("汇总阶段失败: %w", err)
	}

	sendProgress(pw, "report_done", map[string]any{"markdown_size": len(markdown)})
	result, _ := json.Marshal(map[string]string{"markdown": markdown})
	return json.RawMessage(result), nil
}

// --- Phase 1: Analyst ---

var analystSystemPrompt = `你是一个数据分析师。根据用户需求，规划需要哪些图表。
你只能输出以下图表类型：

- bar: 柱状图，字段：type="bar", title, x_label（横轴标签）, y_label（纵轴标签）, colors（可选，如 ["#2196F3"]）
- line: 折线图，字段：type="line", title, x_label, y_label, colors
- pie: 饼图，字段：type="pie", title, colors（x_label/y_label 不需要）
- table: 表格，字段：type="table", title, x_label（表头说明）, y_label（数据列说明）

输出 JSON 数组格式示例：
[{"type":"bar","title":"各行业企业数","x_label":"行业","y_label":"数量","colors":["#2196F3"]},
 {"type":"pie","title":"载体规模分布","colors":["#FF9800","#4CAF50"]}]

只输出 JSON 数组，不要其他内容。`

func (t *GenerateReport) runAnalyst(ctx context.Context, prompt string) ([]ChartSpec, error) {
	specs, err := chatAndParse[[]ChartSpec](t.ai, ctx, "analyst", analystSystemPrompt, prompt, "分析阶段解析失败")
	if err != nil {
		return nil, err
	}
	if specs == nil {
		return nil, nil
	}
	return *specs, nil
}

// --- Phase 2: Executor ---

// executorPrompts 每种图表类型的 Executor 提示词。
var executorPrompts = map[string]string{
	"bar": `你需要为柱状图准备数据。每根柱子代表一个类别，高度代表数值。
你需要提供 labels（字符串数组）和 data（数字数组）。
可用查询参数包括 group_by、aggregate、group_by_period、filters 等，请根据图表需求选择合适的方案。`,

	"line": `你需要为折线图准备数据。x轴通常为时间序列或类别，y轴为数值。
你需要提供 labels（字符串数组）和 data（数字数组）。
可用查询参数包括 group_by、group_by_period（推荐用于趋势）、aggregate、filters 等。`,

	"pie": `你需要为饼图准备数据。每一扇代表一个类别的占比或计数。
你需要提供 labels（字符串数组）和 data（数字数组）。
可用查询参数包括 group_by、aggregate、filters 等。`,

	"table": `你需要为表格准备数据，展示明细记录。
你需要提供 columns（表头数组）和 rows（行数据数组）。
通常不需要聚合，直接查询原始记录，可设置 limit 控制行数。`,
}

func (t *GenerateReport) buildTableList() string {
	var sb strings.Builder
	for _, c := range t.configs {
		sb.WriteString("- ")
		sb.WriteString(c.TblName())
		sb.WriteString(": ")
		sb.WriteString(c.Description)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t *GenerateReport) runExecutor(ctx context.Context, specs []ChartSpec, pw agent.ProgressWriter) ([]ReportChart, error) {
	results := make([]ReportChart, len(specs))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	tableList := t.buildTableList()

	for i, spec := range specs {
		g.Go(func() error {
			// 根据图表类型选择提示词
			typePrompt := executorPrompts[spec.Type]
			if typePrompt == "" {
				typePrompt = executorPrompts["bar"] // fallback
			}
			execPrompt := typePrompt + "\n\n可用表及查询参数：\n" + tableList + "\n输出 JSON：{\"table\":\"query_xxx\",\"params\":{...},\"data_mapping\":{\"x_field\":\"...\",\"y_field\":\"...\"}}。只输出 JSON。"

			// Step 1: AI 决定查询方案
			chartDesc := fmt.Sprintf("图表类型：%s，标题：%s", spec.Type, spec.Title)
			if spec.XLabel != "" {
				chartDesc += fmt.Sprintf("，横轴：%s", spec.XLabel)
			}
			if spec.YLabel != "" {
				chartDesc += fmt.Sprintf("，纵轴：%s", spec.YLabel)
			}
			plan, err := chatAndParse[QueryPlan](t.ai, ctx, "executor-plan", execPrompt, chartDesc, "执行阶段解析查询方案失败")
			if err != nil {
				return fmt.Errorf("规划查询失败[%s]: %w", spec.Title, err)
			}

			// Step 2: 执行查询
			var cfg *TableConfig
			for _, c := range t.configs {
				if c.TblName() == plan.Table {
					cfg = c
					break
				}
			}
			if cfg == nil {
				return fmt.Errorf("未知表[%s]: %s", spec.Title, plan.Table)
			}

			qr, err := t.engine.Query(t.db, cfg, plan.Params)
			if err != nil {
				return fmt.Errorf("查询失败[%s]: %w", spec.Title, err)
			}

			// Step 3: 提取数据
			labels, data := extractChartData(qr, plan.DataMapping)

			// Step 4: 调用 MCP 生成图表
			mcpClient, err := mcpchart.NewClient()
			if err != nil {
				return fmt.Errorf("启动图表服务失败[%s]: %w", spec.Title, err)
			}
			defer mcpClient.Close()

			chartResult, err := mcpClient.Render(map[string]any{
				"type":   spec.Type,
				"title":  spec.Title,
				"xlabel": spec.XLabel,
				"ylabel": spec.YLabel,
				"colors": spec.Colors,
				"labels": labels,
				"data":   []any{data},
			})
			if err != nil {
				return fmt.Errorf("图表生成失败[%s]: %w", spec.Title, err)
			}

			// Step 5: 生成 Markdown 表格（数据展示）
			dataTable := buildMarkdownTable(qr)

			results[i] = ReportChart{
				Spec:     spec,
				ImageURL: fmt.Sprintf("/api/v1/files/chart/%s", chartResult.FileName),
				Data:     dataTable,
			}

			sendProgress(pw, "report_progress", map[string]any{
				"phase":   "executor",
				"current": i + 1,
				"total":   len(specs),
				"title":   spec.Title,
			})
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// extractChartData 从查询结果中提取图表需要的 labels 和 data。
func extractChartData(qr *QueryResult, dm DataMapping) ([]string, []float64) {
	var labels []string
	var data []float64
	for _, row := range qr.Rows {
		for j, col := range qr.Columns {
			if col == dm.XField && j < len(row) {
				labels = append(labels, fmt.Sprintf("%v", row[j]))
			}
			if col == dm.YField && j < len(row) {
				var v float64
				fmt.Sscanf(fmt.Sprintf("%v", row[j]), "%f", &v)
				data = append(data, v)
			}
		}
	}
	return labels, data
}

// buildMarkdownTable 将查询结果转为 Markdown 表格。
func buildMarkdownTable(qr *QueryResult) string {
	if len(qr.Columns) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("| ")
	sb.WriteString(strings.Join(qr.Columns, " | "))
	sb.WriteString(" |\n|")
	sb.WriteString(strings.Repeat("---|", len(qr.Columns)))
	sb.WriteString("\n")
	for _, row := range qr.Rows {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = fmt.Sprintf("%v", v)
		}
		sb.WriteString("| ")
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteString(" |\n")
	}
	return sb.String()
}

// --- Phase 3: Summarizer ---

var summarizerSystemPrompt = `你是一个数据分析报告撰写助手。根据用户需求和已生成的图表，撰写一份完整的数据分析报告（Markdown 格式）。
要求：包含标题、摘要、各数据章节（每个图表单独一节并引用图片）、总结与建议。
只输出 Markdown 报告，不要其他解释。`

func (t *GenerateReport) runSummarizer(ctx context.Context, prompt string, charts []ReportChart) (string, error) {
	var sb strings.Builder
	sb.WriteString("用户需求: ")
	sb.WriteString(prompt)
	sb.WriteString("\n\n")
	for i, c := range charts {
		fmt.Fprintf(&sb, "## 图表 %d: %s\n\n", i+1, c.Spec.Title)
		fmt.Fprintf(&sb, "![](%s)\n\n", c.ImageURL)
		if c.Data != "" {
			sb.WriteString("**数据明细：**\n\n")
			sb.WriteString(c.Data)
			sb.WriteString("\n\n")
		}
	}

	md, err := chatAndParse[string](t.ai, ctx, "summarizer", summarizerSystemPrompt, sb.String(), "汇总阶段解析失败")
	if err != nil {
		return "", err
	}
	if md == nil {
		return "", fmt.Errorf("AI 返回为空")
	}
	return *md, nil
}

// --- helpers ---

func sendProgress(pw agent.ProgressWriter, typ string, data map[string]any) {
	if pw != nil {
		pw(typ, data)
	}
}

// chatAndParse 本地 AI 调用辅助函数（避免对 service 包的硬依赖）。
func chatAndParse[T any](ai *aiclient.Client, ctx context.Context, op, system, user, parseErrMsg string) (*T, error) {
	resp, err := ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	})
	if err != nil {
		slog.Warn("AI chat failed", "op", op, "error", err)
		return nil, fmt.Errorf("AI服务暂不可用")
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI返回为空")
	}
	text := resp.Choices[0].Message.Content
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var result T
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		slog.Error("AI parse failed", "op", op, "error", err, "text", text[:min(len(text), 200)])
		return nil, errors.New(parseErrMsg)
	}
	return &result, nil
}
