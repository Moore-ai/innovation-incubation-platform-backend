package builtin

// ChartPlan 分析师输出的完整图表计划（含查询方案）。
type ChartPlan struct {
	Type        string         `json:"type"`                  // bar | line | pie | table | gantt | quadrantChart | timeline | flowchart | sankey-beta
	Title       string         `json:"title"`                 // 图表标题
	XLabel      string         `json:"x_label,omitempty"`     // 横轴/分类标签
	YLabel      string         `json:"y_label,omitempty"`     // 纵轴/数值标签
	Colors      []string       `json:"colors,omitempty"`      // 可选配色
	Table       string         `json:"table"`                 // 如 "query_enterprises"
	Params      map[string]any `json:"params"`                // 如 {"group_by":"industry","aggregate":"count"}
	DataMapping DataMapping    `json:"data_mapping"`          // 数据→图表轴的映射
}

// ChartSpec 图表需求描述（Analyst 输出）。
type ChartSpec struct {
	Type   string   `json:"type"`    // bar | line | pie | table | gantt | quadrantChart | timeline | flowchart | sankey-beta
	Title  string   `json:"title"`   // 图表标题
	XLabel string   `json:"x_label"` // 横轴/分类标签
	YLabel string   `json:"y_label"` // 纵轴/数值标签
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
	XField      string `json:"x_field"`                // 通用标签/分类字段
	YField      string `json:"y_field"`                // 通用数值字段
	ZField      string `json:"z_field,omitempty"`      // quadrantChart y轴值 / sankey value
	StartField  string `json:"start_field,omitempty"`  // gantt 开始日期
	EndField    string `json:"end_field,omitempty"`    // gantt 结束日期
	SourceField string `json:"source_field,omitempty"` // sankey 源节点
	TargetField string `json:"target_field,omitempty"` // sankey 目标节点
	GroupField  string `json:"group_field,omitempty"`  // gantt section / timeline section
}

// ReportChart 执行器完成后的图表结果（传给 Summarizer）。
type ReportChart struct {
	Spec    ChartSpec
	Mermaid string // Mermaid 代码块（含 ```mermaid 围栏）
}
