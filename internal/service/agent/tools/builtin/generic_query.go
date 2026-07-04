package builtin

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*GenericQueryTool)(nil)

type GenericQueryTool struct {
	cfg    *TableConfig
	db     *gorm.DB
	engine *QueryEngine
}

func NewGenericQueryTool(cfg *TableConfig, db *gorm.DB) *GenericQueryTool {
	return &GenericQueryTool{cfg: cfg, db: db, engine: &QueryEngine{}}
}

func (t *GenericQueryTool) Name() string          { return t.cfg.Table }
func (t *GenericQueryTool) Description() string   { return t.cfg.Description }
func (t *GenericQueryTool) AllowedRoles() []string { return []string{"government"} }

func (t *GenericQueryTool) InputSchema() json.RawMessage {
	var props []string
	for _, col := range t.cfg.Columns {
		desc := col.Name
		switch col.FilterMode {
		case "like":
			desc += "（模糊匹配）"
		case "range":
			continue // 范围字段另外生成 from/to
		}
		if len(col.EnumValues) > 0 {
			desc += "，可选值：" + strings.Join(col.EnumValues, "/")
		}
		props = append(props, `"`+col.Name+`":{"type":"`+col.Type+`","description":"`+desc+`"}`)
	}
	// 范围字段生成 from/to
	for _, col := range t.cfg.Columns {
		if col.FilterMode == "range" {
			props = append(props, `"`+col.Name+`_from":{"type":"string","description":"`+col.Name+` 起始 YYYY-MM-DD"}`)
			props = append(props, `"`+col.Name+`_to":{"type":"string","description":"`+col.Name+` 截止 YYYY-MM-DD"}`)
		}
	}
	// 通用参数
	props = append(props, `"group_by":{"type":"string","description":"分组字段"}`)
	props = append(props, `"group_by_period":{"type":"string","enum":["day","week","month","quarter","year"],"description":"时间分桶"}`)
	if len(t.cfg.Aggregates) > 0 {
		props = append(props, `"aggregate":{"type":"string","enum":["`+strings.Join(t.cfg.Aggregates, `","`)+`"],"description":"聚合方式"}`)
	}
	props = append(props, `"order_by":{"type":"string","description":"排序字段，-前缀降序"}`)
	props = append(props, `"limit":{"type":"integer","description":"返回条数，默认100，最大2000"}`)

	schema := `{"type":"object","properties":{` + strings.Join(props, ",") + `}}`
	return json.RawMessage(schema)
}

func (t *GenericQueryTool) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"columns":{"type":"array","items":{"type":"string"}},"rows":{"type":"array","items":{"type":"array"}},"row_count":{"type":"integer"}}}`)
}

func (t *GenericQueryTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	result, err := t.engine.Query(t.db, t.cfg, params)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	return json.RawMessage(b), nil
}

var TableConfigs = []*TableConfig{
	{
		Table: "query_enterprises",
		Description: "查询企业列表。支持按名称/信用代码/行业/规模/地址/入驻时间筛选，支持分组统计。\n参数：name(模糊)/credit_code(精确)/industry(精确)/scale(精确)/address(模糊)/created_from/created_to/group_by(industry,scale)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "name", Type: "string", FilterMode: "like"},
			{Name: "credit_code", Type: "string", FilterMode: "exact"},
			{Name: "industry", Type: "string", FilterMode: "exact"},
			{Name: "scale", Type: "string", FilterMode: "exact"},
			{Name: "address", Type: "string", FilterMode: "like"},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_carriers",
		Description: "查询载体列表。支持按名称/类型/区域/规模/创建时间筛选，支持分组统计。\n参数：name(模糊)/type(精确)/area(精确)/scale(small/medium/large)/created_from/created_to/group_by(area,type,scale)/group_by_period/aggregate/order_by(-incubation_count)/limit",
		Columns: []ColumnDef{
			{Name: "name", Type: "string", FilterMode: "like"},
			{Name: "type", Type: "string", FilterMode: "exact"},
			{Name: "area", Type: "string", FilterMode: "exact"},
			{Name: "scale", Type: "string", FilterMode: "exact", EnumValues: []string{"small", "medium", "large"}},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_policies",
		Description: "查询政策列表。支持按标题/目标角色/部门/状态/有效期筛选，支持分组统计。\n参数：title(模糊)/target_role(enterprise/carrier/both)/department(精确)/status(draft/published/closed)/start_from/start_to/end_from/end_to/group_by(department,target_role,status)/group_by_period/aggregate/order_by(-published_at)/limit",
		TimeColumn: "published_at",
		Columns: []ColumnDef{
			{Name: "title", Type: "string", FilterMode: "like"},
			{Name: "target_role", Type: "string", FilterMode: "exact", EnumValues: []string{"enterprise", "carrier", "both"}},
			{Name: "department", Type: "string", FilterMode: "exact"},
			{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "published", "closed"}},
			{Name: "start_date", Type: "string", FilterMode: "range"},
			{Name: "end_date", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_policy_applications",
		Description: "查询政策申报记录。按政策/申请人类型/状态/提交时间筛选，支持分组统计。\n参数：policy_id(精确)/applicant_type(enterprise/carrier)/status(draft/pending/carrier_review/gov_review/approved/rejected/returned)/created_from/created_to/group_by(status,applicant_type)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "policy_id", Type: "integer", FilterMode: "exact"},
			{Name: "applicant_type", Type: "string", FilterMode: "exact", EnumValues: []string{"enterprise", "carrier"}},
			{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "carrier_review", "gov_review", "approved", "rejected", "returned"}},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_incubation_records",
		Description: "查询孵化记录。按企业/载体/孵化状态/审核状态/入驻时间筛选，支持分组统计。\n参数：enterprise_id/carrier_id/incubate_status(in_incubation/graduated)/status/start_from/start_to/group_by(carrier_id,incubate_status,status)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "enterprise_id", Type: "integer", FilterMode: "exact"},
			{Name: "carrier_id", Type: "integer", FilterMode: "exact"},
			{Name: "incubate_status", Type: "string", FilterMode: "exact", EnumValues: []string{"in_incubation", "graduated"}},
			{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "approved", "rejected", "returned"}},
			{Name: "incubate_start", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_major_changes",
		Description: "查询重大变更记录。按企业/变更类型/状态/时间筛选，支持分组统计。\n参数：enterprise_id/change_type(模糊)/status/created_from/created_to/group_by(status,change_type)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "enterprise_id", Type: "integer", FilterMode: "exact"},
			{Name: "change_type", Type: "string", FilterMode: "like"},
			{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "approved", "rejected", "returned"}},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_performance_campaigns",
		Description: "查询考核活动。按名称/年份/是否活跃筛选。\n参数：name(模糊)/year(精确)/is_active/created_from/created_to/group_by(year)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "name", Type: "string", FilterMode: "like"},
			{Name: "year", Type: "integer", FilterMode: "exact"},
			{Name: "is_active", Type: "boolean", FilterMode: "exact"},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_performance_submissions",
		Description: "查询考核提交记录。按活动/载体/状态/评分/时间筛选，支持对 score 求平均/最大/最小。\n参数：campaign_id/carrier_id/status/has_score/created_from/created_to/group_by(status,campaign_id)/group_by_period/aggregate(count,avg,max,min)/order_by(-score)/limit",
		Columns: []ColumnDef{
			{Name: "campaign_id", Type: "integer", FilterMode: "exact"},
			{Name: "carrier_id", Type: "integer", FilterMode: "exact"},
			{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "approved", "rejected", "returned"}},
			{Name: "has_score", Type: "boolean", FilterMode: "exact"},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count", "avg", "max", "min"},
	},
	{
		Table: "query_appeals",
		Description: "查询诉求记录。按问题类型/部门/状态/申请人类型/时间筛选，支持分组统计。\n参数：problem_type(tax/financing/property/utility/registration/labor/construction/supervision/reward/other)/department(精确)/status(pending/processed)/applicant_type(enterprise/carrier)/created_from/created_to/group_by(problem_type,department,status,applicant_type)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "problem_type", Type: "string", FilterMode: "exact", EnumValues: []string{"tax", "financing", "property", "utility", "registration", "labor", "construction", "supervision", "reward", "other"}},
			{Name: "department", Type: "string", FilterMode: "exact"},
			{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"pending", "processed"}},
			{Name: "applicant_type", Type: "string", FilterMode: "exact", EnumValues: []string{"enterprise", "carrier"}},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
	{
		Table: "query_approvals",
		Description: "查询审核操作记录。按目标类型/步骤/动作/审核人/时间筛选，支持分组统计。\n参数：target_type(incubation/major_change/policy/performance/account_deletion)/step(carrier_review/gov_review)/action(submit/approve/reject/return)/reviewer_id/created_from/created_to/group_by(target_type,step,action)/group_by_period/aggregate/order_by/limit",
		Columns: []ColumnDef{
			{Name: "target_type", Type: "string", FilterMode: "exact", EnumValues: []string{"incubation", "major_change", "policy", "performance", "account_deletion"}},
			{Name: "step", Type: "string", FilterMode: "exact", EnumValues: []string{"carrier_review", "gov_review"}},
			{Name: "action", Type: "string", FilterMode: "exact", EnumValues: []string{"submit", "approve", "reject", "return"}},
			{Name: "reviewer_id", Type: "integer", FilterMode: "exact"},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
		Aggregates: []string{"count"},
	},
}
