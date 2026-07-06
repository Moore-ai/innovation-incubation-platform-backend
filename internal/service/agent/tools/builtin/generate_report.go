package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"gorm.io/gorm"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/pkg/aiclient"

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

	markdown = cleanMarkdown(markdown)
	sendProgress(pw, "report_done", map[string]any{"markdown_size": len(markdown)})
	result, _ := json.Marshal(map[string]string{"markdown": markdown})
	return json.RawMessage(result), nil
}

// --- Phase 1: Analyst ---

var analystSystemPrompt = `你是一个数据分析师。根据用户需求，规划需要哪些图表。
你只能输出以下图表类型：

- bar: 柱状图，字段：type="bar", title, x_label（横轴标签）, y_label（纵轴标签）, colors（可选）
- line: 折线图，字段：type="line", title, x_label, y_label, colors
- pie: 饼图，字段：type="pie", title, colors（x_label/y_label 不需要）
- table: 表格，字段：type="table", title, x_label（表头说明）, y_label（数据列说明）
- gantt: 甘特图/时间线，适合展示任务或事件的时间跨度。字段：type="gantt", title, x_label（任务列）, y_label（时间范围描述）
- quadrantChart: 四象限分析图，适合两个维度的交叉分析（如规模×增速）。字段：type="quadrantChart", title, x_label（x轴含义）, y_label（y轴含义）
- timeline: 事件时间线，按时间顺序展示关键事件。字段：type="timeline", title
- flowchart: 流程图，适合展示流程、层级关系、分类结构。字段：type="flowchart", title, x_label（可选方向如 TD/LR）
- sankey-beta: 流向图，适合展示资源/企业的来源去向分布。字段：type="sankey-beta", title

输出 JSON 数组格式示例：
[{"type":"bar","title":"各行业企业数","x_label":"行业","y_label":"数量","colors":["#2196F3"]},
 {"type":"pie","title":"载体规模分布","colors":["#FF9800","#4CAF50"]},
 {"type":"gantt","title":"企业入驻时间线","x_label":"企业名称","y_label":"2025-2026"}]

只输出 JSON 数组，不要其他内容。`

func (t *GenerateReport) runAnalyst(ctx context.Context, prompt string) ([]ChartSpec, error) {
	specs, err := chatAndParse[[]ChartSpec](t.ai, ctx, analystSystemPrompt, prompt, "分析阶段解析失败")
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
可用查询参数包括 group_by、aggregate、group_by_period、filters 等。`,

	"line": `你需要为折线图准备数据。x轴通常为时间序列或类别，y轴为数值。
你需要提供 labels（字符串数组）和 data（数字数组）。
可用查询参数包括 group_by、group_by_period（推荐用于趋势）、aggregate、filters 等。`,

	"pie": `你需要为饼图准备数据。每一扇代表一个类别的占比或计数。
你需要提供 labels（字符串数组）和 data（数字数组）。
可用查询参数包括 group_by、aggregate、filters 等。`,

	"table": `你需要为表格准备数据，展示明细记录。
你需要提供 columns（表头数组）和 rows（行数据数组）。
通常不需要聚合，直接查询原始记录，可设置 limit 控制行数。`,

	"gantt": `你需要为甘特图准备数据，展示任务或事件的时间跨度。
每个任务需要：名称（x_field）、开始日期（start_field）、结束日期（end_field），可选分组（group_field）。
日期字段格式需为 YYYY-MM-DD。`,

	"quadrantChart": `你需要为四象限分析图准备数据，展示两个维度的交叉分析。
每个点需要：标签（x_field）、x轴数值（y_field）、y轴数值（z_field）。
x_field 取分类标签列，y_field 取 x 轴指标列，z_field 取 y 轴指标列。`,

	"timeline": `你需要为事件时间线准备数据。
每个事件需要：时间段名称（x_field）、事件描述（y_field），可选分类（group_field）。
x_field 通常是年份或月份，y_field 是事件文本。`,

	"flowchart": `你需要为流程图准备数据，展示流程步骤、层级关系或分类结构。
查询相关数据后，还需要规划流程图的具体结构（节点和连线关系）。
如果你的查询已覆盖所需数据，请按正常格式返回 QueryPlan。`,

	"sankey-beta": `你需要为流向图准备数据，展示从源到目标的流量分布。
每行需要：源节点（source_field）、目标节点（target_field）、流量值（z_field）。
可用查询参数包括 group_by、aggregate、filters 等。`,
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

	// 单 goroutine channel 串行化进度推送，避免并发写 SSE
	type prog struct {
		idx   int
		title string
	}
	progressCh := make(chan prog, len(specs))
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		done := 0
		for p := range progressCh {
			done++
			sendProgress(pw, "report_progress", map[string]any{
				"phase":   "executor",
				"current": done,
				"total":   len(specs),
				"title":   p.title,
			})
		}
	}()

	dataMappingHint := "\n输出 JSON：{\"table\":\"query_xxx\",\"params\":{...},\"data_mapping\":{\"x_field\":\"...\",\"y_field\":\"...\""
	dataMappingHint += ",\"z_field\"(quadrantChart/sankey),\"" + "start_field\"(gantt)," + "\"end_field\"(gantt)," + "\"source_field\"(sankey)," + "\"target_field\"(sankey)," + "\"group_field\"(gantt/timeline)}}\n按需填写扩展字段，不需要的不要填。只输出 JSON。"

	for i, spec := range specs {
		g.Go(func() error {
			typePrompt := executorPrompts[spec.Type]
			if typePrompt == "" {
				typePrompt = executorPrompts["bar"]
			}
			execPrompt := typePrompt + "\n\n可用表及查询参数：\n" + tableList + dataMappingHint

			chartDesc := fmt.Sprintf("图表类型：%s，标题：%s", spec.Type, spec.Title)
			if spec.XLabel != "" {
				chartDesc += fmt.Sprintf("，横轴：%s", spec.XLabel)
			}
			if spec.YLabel != "" {
				chartDesc += fmt.Sprintf("，纵轴：%s", spec.YLabel)
			}
			plan, err := chatAndParse[QueryPlan](t.ai, ctx, execPrompt, chartDesc, "执行阶段解析查询方案失败")
			if err != nil {
				return fmt.Errorf("规划查询失败[%s]: %w", spec.Title, err)
			}

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

			var mermaid string
			if spec.Type == "flowchart" {
				mermaid, err = buildMermaidFlowchart(t.ai, ctx, spec, qr)
				if err != nil {
					return fmt.Errorf("流程图生成失败[%s]: %w", spec.Title, err)
				}
			} else {
				mermaid = buildMermaid(spec, qr, plan.DataMapping)
			}

			results[i] = ReportChart{
				Spec:    spec,
				Mermaid: mermaid,
			}

			progressCh <- prog{idx: i, title: spec.Title}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		close(progressCh)
		<-progressDone
		return nil, err
	}
	close(progressCh)
	<-progressDone
	return results, nil
}

// extractChartData 从查询结果中提取图表需要的 labels 和 data，跳过空值行。
func extractChartData(qr *QueryResult, dm DataMapping) ([]string, []float64) {
	var labels []string
	var data []float64
	for _, row := range qr.Rows {
		if len(row) == 0 {
			continue
		}
		var label string
		var val float64
		hasVal := false
		for j, col := range qr.Columns {
			if col == dm.XField && j < len(row) && row[j] != nil {
				label = fmt.Sprintf("%v", row[j])
			}
			if col == dm.YField && j < len(row) && row[j] != nil {
				n, err := fmt.Sscanf(fmt.Sprintf("%v", row[j]), "%f", &val)
				if err == nil && n == 1 {
					hasVal = true
				}
			}
		}
		if label != "" && hasVal {
			labels = append(labels, label)
			data = append(data, val)
		}
	}
	return labels, data
}

// --- Phase 3: Summarizer ---

var summarizerSystemPrompt = `你是一个数据分析报告撰写助手。根据用户需求和已生成的图表，撰写一份完整的数据分析报告（Markdown 格式）。
要求：包含标题、摘要、各数据章节（每个图表单独一节，原样保留提供的 Mermaid 代码块）、总结与建议。
只输出 Markdown 报告，不要其他解释。`

func (t *GenerateReport) runSummarizer(ctx context.Context, prompt string, charts []ReportChart) (string, error) {
	var sb strings.Builder
	sb.WriteString("用户需求: ")
	sb.WriteString(prompt)
	sb.WriteString("\n\n")
	for i, c := range charts {
		fmt.Fprintf(&sb, "## 图表 %d: %s\n\n", i+1, c.Spec.Title)
		sb.WriteString(c.Mermaid)
		sb.WriteString("\n\n")
	}

	resp, err := t.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: summarizerSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: sb.String()},
		},
	})
	if err != nil {
		return "", fmt.Errorf("汇总阶段 AI 调用失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 返回为空")
	}
	return resp.Choices[0].Message.Content, nil
}

// buildMermaid 根据图表类型从查询结果构建 Mermaid 代码块。
func buildMermaid(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	switch spec.Type {
	case "pie":
		return buildMermaidPie(spec, qr, dm)
	case "bar", "line":
		return buildMermaidXYChart(spec, qr, dm)
	case "table":
		return buildMarkdownTable(spec, qr)
	case "gantt":
		return buildMermaidGantt(spec, qr, dm)
	case "quadrantChart":
		return buildMermaidQuadrant(spec, qr, dm)
	case "timeline":
		return buildMermaidTimeline(spec, qr, dm)
	case "sankey-beta":
		return buildMermaidSankey(spec, qr, dm)
	default:
		return buildMermaidPie(spec, qr, dm)
	}
}

func buildMermaidPie(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	labels, data := extractChartData(qr, dm)
	var sb strings.Builder
	sb.WriteString("```mermaid\npie")
	if spec.Title != "" {
		sb.WriteString(" title ")
		sb.WriteString(spec.Title)
	}
	sb.WriteString("\n")
	for i, label := range labels {
		var val float64
		if i < len(data) {
			val = data[i]
		}
		fmt.Fprintf(&sb, "    \"%s\" : %.0f\n", label, val)
	}
	sb.WriteString("```")
	return sb.String()
}

func buildMermaidXYChart(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	labels, data := extractChartData(qr, dm)
	if len(data) == 0 {
		return ""
	}
	maxVal := data[0]
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	top := maxVal * 1.2
	if top == 0 {
		top = 10
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\nxychart-beta\n")
	if spec.Title != "" {
		fmt.Fprintf(&sb, "    title \"%s\"\n", spec.Title)
	}
	sb.WriteString("    x-axis [")
	for i, l := range labels {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("\"")
		sb.WriteString(l)
		sb.WriteString("\"")
	}
	sb.WriteString("]\n")
	yLabel := spec.YLabel
	if yLabel == "" {
		yLabel = "数量"
	}
	fmt.Fprintf(&sb, "    y-axis \"%s\" 0 --> %.0f\n", yLabel, math.Ceil(top))
	dataStr := make([]string, len(data))
	for i, v := range data {
		dataStr[i] = fmt.Sprintf("%.0f", v)
	}
	if spec.Type == "line" {
		sb.WriteString("    line [")
	} else {
		sb.WriteString("    bar [")
	}
	sb.WriteString(strings.Join(dataStr, ", "))
	sb.WriteString("]\n```")
	return sb.String()
}

func buildMarkdownTable(spec ChartSpec, qr *QueryResult) string {
	var sb strings.Builder
	if spec.Title != "" {
		sb.WriteString("**")
		sb.WriteString(spec.Title)
		sb.WriteString("**\n\n")
	}
	for i, col := range qr.Columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(col)
	}
	sb.WriteString("\n")
	for i := range qr.Columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString("---")
	}
	sb.WriteString("\n")
	for _, row := range qr.Rows {
		for i, cell := range row {
			if i > 0 {
				sb.WriteString(" | ")
			}
			if cell == nil {
				sb.WriteString("-")
			} else {
				fmt.Fprintf(&sb, "%v", cell)
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

func buildMermaidGantt(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\ngantt\n")
	if spec.Title != "" {
		fmt.Fprintf(&sb, "    title %s\n", spec.Title)
	}
	sb.WriteString("    dateFormat YYYY-MM-DD\n")

	type ganttRow struct {
		task, start, end, section string
	}
	var rows []ganttRow
	for _, row := range qr.Rows {
		r := ganttRow{
			task:    extractStrField(qr, row, dm.XField),
			start:   extractStrField(qr, row, dm.StartField),
			end:     extractStrField(qr, row, dm.EndField),
			section: extractStrField(qr, row, dm.GroupField),
		}
		if r.task != "" && r.start != "" {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return ""
	}

	seenSection := map[string]bool{}
	for _, r := range rows {
		if r.section != "" && !seenSection[r.section] {
			seenSection[r.section] = true
			fmt.Fprintf(&sb, "    section %s\n", r.section)
		}
		duration := r.end
		if duration == "" {
			duration = "30d"
		}
		fmt.Fprintf(&sb, "    %s :%s, %s\n", r.task, r.start, duration)
	}
	sb.WriteString("```")
	return sb.String()
}

func buildMermaidQuadrant(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	type quadPoint struct {
		label string
		x, y  float64
	}
	var pts []quadPoint
	for _, row := range qr.Rows {
		label := extractStrField(qr, row, dm.XField)
		var xv, yv float64
		fmt.Sscanf(extractStrField(qr, row, dm.YField), "%f", &xv)
		fmt.Sscanf(extractStrField(qr, row, dm.ZField), "%f", &yv)
		if label != "" {
			pts = append(pts, quadPoint{label, xv, yv})
		}
	}
	if len(pts) < 2 {
		return ""
	}
	// 归一化到 [0,1]
	var maxX, maxY float64
	for _, p := range pts {
		if p.x > maxX {
			maxX = p.x
		}
		if p.y > maxY {
			maxY = p.y
		}
	}
	if maxX == 0 {
		maxX = 1
	}
	if maxY == 0 {
		maxY = 1
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\nquadrantChart\n")
	if spec.Title != "" {
		fmt.Fprintf(&sb, "    title %s\n", spec.Title)
	}
	xLabel := spec.XLabel
	if xLabel == "" {
		xLabel = "x轴"
	}
	yLabel := spec.YLabel
	if yLabel == "" {
		yLabel = "y轴"
	}
	fmt.Fprintf(&sb, "    x-axis \"低%s\" --> \"高%s\"\n", xLabel, xLabel)
	fmt.Fprintf(&sb, "    y-axis \"低%s\" --> \"高%s\"\n", yLabel, yLabel)
	sb.WriteString("    quadrant-1 \"高x高y\"\n    quadrant-2 \"低x高y\"\n    quadrant-3 \"低x低y\"\n    quadrant-4 \"高x低y\"\n")
	for _, p := range pts {
		fmt.Fprintf(&sb, "    \"%s\": [%.2f, %.2f]\n", p.label, p.x/maxX, p.y/maxY)
	}
	sb.WriteString("```")
	return sb.String()
}

func buildMermaidTimeline(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	type tlEvent struct {
		period, event, section string
	}
	var events []tlEvent
	for _, row := range qr.Rows {
		e := tlEvent{
			period:  extractStrField(qr, row, dm.XField),
			event:   extractStrField(qr, row, dm.YField),
			section: extractStrField(qr, row, dm.GroupField),
		}
		if e.period != "" && e.event != "" {
			events = append(events, e)
		}
	}
	if len(events) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("```mermaid\ntimeline\n")
	if spec.Title != "" {
		fmt.Fprintf(&sb, "    title %s\n", spec.Title)
	}

	// 按 section 分组
	type group struct {
		section string
		items   []tlEvent
	}
	var groups []group
	groupIdx := map[string]int{}
	for _, e := range events {
		if idx, ok := groupIdx[e.section]; ok {
			groups[idx].items = append(groups[idx].items, e)
		} else {
			groupIdx[e.section] = len(groups)
			groups = append(groups, group{e.section, []tlEvent{e}})
		}
	}
	for _, g := range groups {
		if g.section != "" {
			fmt.Fprintf(&sb, "    section %s\n", g.section)
		}
		// 合并同 period 的事件
		periodItems := map[string][]string{}
		for _, e := range g.items {
			periodItems[e.period] = append(periodItems[e.period], e.event)
		}
		for period, items := range periodItems {
			sb.WriteString("    ")
			sb.WriteString(period)
			sb.WriteString(" : ")
			sb.WriteString(strings.Join(items, " : "))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("```")
	return sb.String()
}

func buildMermaidSankey(spec ChartSpec, qr *QueryResult, dm DataMapping) string {
	var sb strings.Builder
	sb.WriteString("```mermaid\n---\nconfig:\n  sankey:\n    showValues: false\n---\nsankey-beta\n")
	if spec.Title != "" {
		sb.WriteString(spec.Title)
		sb.WriteString("\n")
	}

	type flow struct {
		source, target, value string
	}
	var flows []flow
	for _, row := range qr.Rows {
		f := flow{
			source: extractStrField(qr, row, dm.SourceField),
			target: extractStrField(qr, row, dm.TargetField),
			value:  extractStrField(qr, row, dm.ZField),
		}
		if f.source != "" && f.target != "" {
			if f.value == "" {
				f.value = "1"
			}
			flows = append(flows, f)
		}
	}
	if len(flows) == 0 {
		return ""
	}
	for _, f := range flows {
		fmt.Fprintf(&sb, "%s,%s,%s\n", f.source, f.target, f.value)
	}
	sb.WriteString("```")
	return sb.String()
}

// buildMermaidFlowchart 使用 AI 根据查询结果生成流程图 Mermaid。
func buildMermaidFlowchart(ai *aiclient.Client, ctx context.Context, spec ChartSpec, qr *QueryResult) (string, error) {
	var dataDesc strings.Builder
	dataDesc.WriteString("图表标题：")
	dataDesc.WriteString(spec.Title)
	dataDesc.WriteString("\n查询结果（列：")
	dataDesc.WriteString(strings.Join(qr.Columns, ", "))
	dataDesc.WriteString("）：\n")
	for i, row := range qr.Rows {
		if i >= 30 {
			dataDesc.WriteString("...（已截断）\n")
			break
		}
		for j, cell := range row {
			if j > 0 {
				dataDesc.WriteString(" | ")
			}
			if cell != nil {
				fmt.Fprintf(&dataDesc, "%v", cell)
			}
		}
		dataDesc.WriteString("\n")
	}

	direction := spec.XLabel
	if direction == "" {
		direction = "TD"
	}

	systemPrompt := "你是一个流程图生成专家。根据查询结果数据，生成 Mermaid flowchart。\n" +
		"方向用 " + direction + "（TB/LR/TD/RL）。\n" +
		"要求：节点用方括号 [名称] 表示，连接用 --> 表示层级或流向关系。\n" +
		"只输出 mermaid flowchart 代码块，格式：```mermaid\\nflowchart " + direction + "\\n    ...\\n```"

	resp, err := ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: dataDesc.String()},
		},
	})
	if err != nil {
		return "", fmt.Errorf("flowchart AI: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("flowchart AI 返回为空")
	}
	text := resp.Choices[0].Message.Content
	// 确保有 mermaid 围栏
	if !strings.Contains(text, "```mermaid") {
		text = "```mermaid\nflowchart " + direction + "\n" + text + "\n```"
	}
	return text, nil
}

// extractStrField 从行中按字段名提取字符串值。
func extractStrField(qr *QueryResult, row []any, field string) string {
	if field == "" {
		return ""
	}
	for j, col := range qr.Columns {
		if col == field && j < len(row) && row[j] != nil {
			return fmt.Sprintf("%v", row[j])
		}
	}
	return ""
}

func cleanMarkdown(md string) string {
	md = strings.TrimSpace(md)
	if md == "" {
		return md
	}

	if idx := strings.Index(md, "\n# "); idx >= 0 {
		md = md[idx+1:]
	} else if strings.HasPrefix(md, "# ") {
		// 第一行就是标题，无需裁剪
	} else if idx := strings.Index(md, "# "); idx >= 0 {
		md = md[idx:]
	}

	lines := strings.Split(md, "\n")
	lastSep := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "---" {
			lastSep = i
			break
		}
	}
	if lastSep >= 0 && !tailHasStructure(lines[lastSep+1:]) {
		lines = lines[:lastSep]
	}
	for len(lines) > 0 {
		last := len(lines) - 1
		for last >= 0 && strings.TrimSpace(lines[last]) == "" {
			last--
		}
		if last < 0 {
			break
		}
		t := strings.TrimSpace(lines[last])
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "|") ||
			strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "> ") || strings.HasPrefix(t, "**") ||
			isNumberedListItem(t) {
			break
		}
		lines = lines[:last]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func tailHasStructure(tail []string) bool {
	for _, l := range tail {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "|") {
			return true
		}
	}
	return false
}

func isNumberedListItem(line string) bool {
	runes := []rune(line)
	for i := 0; i < len(runes) && runes[i] >= '0' && runes[i] <= '9'; i++ {
		if i+1 < len(runes) && (runes[i+1] == '.' || runes[i+1] == '、') {
			return true
		}
	}
	return false
}

// --- helpers ---

func sendProgress(pw agent.ProgressWriter, typ string, data map[string]any) {
	if pw != nil {
		pw(typ, data)
	}
}

// chatAndParse 本地 AI 调用辅助函数（避免对 service 包的硬依赖）。
func chatAndParse[T any](ai *aiclient.Client, ctx context.Context, system, user, parseErrMsg string) (*T, error) {
	resp, err := ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("AI服务暂不可用")
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI返回为空")
	}
	text := resp.Choices[0].Message.Content
	// 与 service.ChatAndParse 一致的清理逻辑：先搜代码围栏再兜底取 JSON 边界
	for _, prefix := range []string{"```json", "```"} {
		if idx := strings.Index(text, prefix); idx >= 0 {
			text = text[idx+len(prefix):]
			break
		}
	}
	if idx := strings.LastIndex(text, "```"); idx >= 0 {
		text = text[:idx]
	}
	// 兜底：取最外层的 JSON 边界
	if trimmed := strings.TrimLeft(text, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '[' {
		if end := strings.LastIndexByte(text, ']'); end >= 0 {
			text = text[:end+1]
		}
	} else if start := strings.IndexByte(text, '{'); start >= 0 {
		if end := strings.LastIndexByte(text, '}'); end >= start {
			text = text[start : end+1]
		}
	}
	text = strings.TrimSpace(text)

	var result T
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, errors.New(parseErrMsg)
	}
	return &result, nil
}
