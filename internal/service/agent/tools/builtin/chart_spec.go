package builtin

// ChartSpec 图表需求描述（Analyst 输出）。
type ChartSpec struct {
	Type   string   `json:"type"`    // bar | line | pie | table
	Title  string   `json:"title"`   // 图表标题
	XLabel string   `json:"x_label"` // 横轴标签（bar/line/table）
	YLabel string   `json:"y_label"` // 纵轴标签（bar/line/table）
	Colors []string `json:"colors"`  // 可选配色
}

// QueryPlan Executor 为每张图规划的数据查询方案。
type QueryPlan struct {
	Table       string         `json:"table"`        // 如 "query_enterprises"
	Params      map[string]any `json:"params"`       // 如 {"group_by":"industry","aggregate":"count"}
	DataMapping DataMapping    `json:"data_mapping"` // 数据→图表轴的映射
}

// DataMapping 将查询结果的字段映射到图表坐标轴。
type DataMapping struct {
	XField string `json:"x_field"` // 结果中用作 x 轴的字段名
	YField string `json:"y_field"` // 结果中用作 y 轴的字段名
}

// ReportChart 执行器完成后的图表结果（传给 Summarizer）。
type ReportChart struct {
	Spec     ChartSpec // 原始图表需求
	ImageURL string    // PNG 图表 URL
}
