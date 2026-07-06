package builtin

// --- Prompts ---

// analystPrompt 返回分析师系统提示词，含可用数据表列表。
func analystPrompt(tableList string) string {
	return `你是一个数据分析师。根据用户需求，规划需要哪些图表。
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

可用数据表：
` + tableList + `
对每张图表，除了上述图表字段外，还需指定数据查询方案：
- table: 选择上述一个可用数据表
- params: 查询参数（如 group_by, aggregate, filters, limit 等）
- data_mapping: 将查询结果的字段映射到图表轴
  - 通用: x_field（分类/标签字段）、y_field（数值字段）
  - gantt 额外: start_field（开始日期）、end_field（结束日期）、group_field（可选分组）
  - quadrantChart: x_field（标签）、y_field（x 轴数值）、z_field（y 轴数值）
  - timeline 额外: x_field（时间点/时间段）、y_field（事件描述）、group_field（可选分类）
  - sankey-beta 额外: source_field（源节点）、target_field（目标节点）、z_field（流量值）
  - flowchart: 不需要 data_mapping（AI 自动生成结构）

输出 JSON 数组格式示例：
[{"type":"bar","title":"各行业企业数","x_label":"行业","y_label":"数量","colors":["#2196F3"],"table":"query_enterprises","params":{"group_by":"industry","aggregate":"count"},"data_mapping":{"x_field":"industry","y_field":"count"}},
 {"type":"pie","title":"载体规模分布","colors":["#4CAF50","#FF9800"],"table":"query_carriers","params":{"group_by":"scale","aggregate":"count"},"data_mapping":{"x_field":"scale","y_field":"count"}}]

只输出 JSON 数组，不要其他内容。`
}

var summarizerSystemPrompt = `你是一个数据分析报告撰写助手。根据用户需求和已生成的图表，撰写一份完整的数据分析报告（Markdown 格式）。
要求：包含标题、摘要、各数据章节（每个图表单独一节，原样保留提供的 Mermaid 代码块）、总结与建议。
只输出 Markdown 报告，不要其他解释。`
