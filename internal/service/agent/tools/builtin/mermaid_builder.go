package builtin

import (
	"context"
	"fmt"
	"math"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"innovation-incubation-platform-backend/pkg/aiclient"
)

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
		fmt.Fprintf(&sb, "    title \"%s\"\n", spec.Title)
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
		fmt.Fprintf(&sb, "    title \"%s\"\n", spec.Title)
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
		fmt.Fprintf(&sb, "    title \"%s\"\n", spec.Title)
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

// buildMermaidForPlan 根据 ChartPlan 生成 Mermaid 代码块（不含 flowchart）。
func buildMermaidForPlan(plan ChartPlan, qr *QueryResult) string {
	spec := ChartSpec{Type: plan.Type, Title: plan.Title, XLabel: plan.XLabel, YLabel: plan.YLabel, Colors: plan.Colors}
	return buildMermaid(spec, qr, plan.DataMapping)
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
