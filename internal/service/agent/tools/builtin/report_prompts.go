package builtin

// --- Prompts ---

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

var summarizerSystemPrompt = `你是一个数据分析报告撰写助手。根据用户需求和已生成的图表，撰写一份完整的数据分析报告（Markdown 格式）。
要求：包含标题、摘要、各数据章节（每个图表单独一节，原样保留提供的 Mermaid 代码块）、总结与建议。
只输出 Markdown 报告，不要其他解释。`
