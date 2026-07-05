package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/pkg/database"

	agent "innovation-incubation-platform-backend/internal/service/agent"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

func realAIClient(t *testing.T) *aiclient.Client {
	t.Helper()
	loadDotEnvForTest()
	key := os.Getenv("AI_API_KEY")
	url := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")
	if key == "" || url == "" {
		t.Skip("AI_API_KEY or AI_BASE_URL not set")
	}
	if model == "" {
		model = "deepseek-v4-flash"
	}
	return aiclient.New(url, key, model, 60)
}

// buildEngine 构造仅含指定工具的测试 Engine。
func buildEngine(t *testing.T, ai *aiclient.Client, _ *gorm.DB, tools []agenttools.Tool) *agent.Engine {
	t.Helper()
	reg := agenttools.NewToolRegistry()
	for _, tool := range tools {
		reg.Register(tool)
	}
	cfg := config.AgentConfig{
		PublicSSETypes:     []string{"reply", "done", "thinking", "error", "tool_call", "tool_result"},
		MaxSteps:           5,
		ToolTimeoutSec:     30,
		ContextWindow:      1,
		HistoryBudgetRatio: 0,
		Memory:             config.AgentMemoryConfig{SemanticLimit: 0, EpisodicLimit: 0},
		RequestTimeoutSec:  120,
	}
	return agent.NewEngine(ai, reg, agentmemory.NewMemoryManager(nil, nil, nil, nil, cfg), nil, cfg)
}

// runGovQuery 执行一次政务查询，返回调用的工具名和首调参数。
func runGovQuery(t *testing.T, eng *agent.Engine, query string) ([]string, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ctx = agent.WithUserID(agent.WithRole(ctx, "government"), 1)

	result, err := eng.Run(ctx, 0, query, "government", func(agent.SSEEvent) {})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	names := extractCalledTools(result.Messages)
	name, args := extractFirstCallArgs(result.Messages)
	t.Logf("Query: %s", query)
	t.Logf("  Tools: %v", names)
	t.Logf("  First: %s(%s)", name, args)
	if result.FinalReply != "" {
		t.Logf("  Reply: %s", result.FinalReply[:min(120, len(result.FinalReply))])
	}
	return names, name, args
}

func extractCalledTools(msgs []agent.ChatMessageRecord) []string {
	var names []string
	for _, m := range msgs {
		if m.Role == "assistant" && m.ToolCalls != "" {
			var calls []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if json.Unmarshal([]byte(m.ToolCalls), &calls) == nil {
				for _, c := range calls {
					names = append(names, c.Function.Name)
				}
			}
		}
	}
	return names
}

func extractFirstCallArgs(msgs []agent.ChatMessageRecord) (string, string) {
	for _, m := range msgs {
		if m.Role == "assistant" && m.ToolCalls != "" {
			var calls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			}
			if json.Unmarshal([]byte(m.ToolCalls), &calls) == nil && len(calls) > 0 {
				return calls[0].Function.Name, calls[0].Function.Arguments
			}
		}
	}
	return "", ""
}

func openTestDBForTool(t *testing.T) *gorm.DB {
	t.Helper()
	loadDotEnvForTest()
	port := 5432
	if p := os.Getenv("DB_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	db, err := database.NewDB(config.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		SSLMode:  "disable",
		LogLevel: "warn",
	})
	if err != nil {
		t.Skipf("skip: %v", err)
	}
	return db
}

// ====== 测试用例 ======

// TestToolSelection_BasicCount 政务问总数
func TestToolSelection_BasicCount(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_enterprises",
			Description: `查询企业列表。参数：name/industry/scale/address/created_from/created_to/
group_by(industry,scale)/group_by_period(day,week,month,quarter,year)/
aggregate(count)/order_by(-created_at)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "平台共有多少家企业？")
	if name != "query_enterprises" {
		t.Errorf("expected query_enterprises, got %s", name)
	}
	if !strings.Contains(args, "count") {
		t.Errorf("expected aggregate=count in args, got %s", args)
	}
}

// TestToolSelection_GroupByIndustry 分组统计
func TestToolSelection_GroupByIndustry(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_enterprises",
			Description: `查询企业列表。参数：name/industry/scale/address/created_from/created_to/
group_by(industry,scale)/group_by_period(day,week,month,quarter,year)/
aggregate(count)/order_by(-created_at)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "各行业的企业数量是多少？")
	if name != "query_enterprises" {
		t.Errorf("expected query_enterprises, got %s", name)
	}
	if !strings.Contains(args, "industry") || !strings.Contains(args, "count") {
		t.Errorf("expected group_by=industry + aggregate=count, got %s", args)
	}
}

// TestToolSelection_TimeTrend 时间趋势
func TestToolSelection_TimeTrend(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_enterprises",
			Description: `查询企业列表。参数：name/industry/scale/address/created_from/created_to/
group_by(industry,scale)/group_by_period(day,week,month,quarter,year)/
aggregate(count)/order_by(-created_at)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "过去半年里，每个月新入驻的企业有多少？")
	if name != "query_enterprises" {
		t.Errorf("expected query_enterprises, got %s", name)
	}
	if !strings.Contains(args, "month") {
		t.Errorf("expected group_by_period=month, got %s", args)
	}
	if !strings.Contains(args, "count") {
		t.Errorf("expected aggregate=count, got %s", args)
	}
}

// TestToolSelection_StatusFilter 状态筛选
func TestToolSelection_StatusFilter(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_policy_applications",
			Description: `查询政策申报记录。参数：policy_id/applicant_type(enterprise/carrier)/
status(draft/pending/carrier_review/gov_review/approved/rejected/returned)/
created_from/created_to/group_by(status,applicant_type)/
group_by_period/aggregate(count)/order_by(-created_at)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "policy_id", Type: "integer", FilterMode: "exact"},
				{Name: "applicant_type", Type: "string", FilterMode: "exact", EnumValues: []string{"enterprise", "carrier"}},
				{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "carrier_review", "gov_review", "approved", "rejected", "returned"}},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "已经审批通过的政策申报有多少？")
	if name != "query_policy_applications" {
		t.Errorf("expected query_policy_applications, got %s", name)
	}
	if !strings.Contains(args, "approved") {
		t.Errorf("expected status=approved, got %s", args)
	}
}

// TestToolSelection_MultiParam 多过滤条件
func TestToolSelection_MultiParam(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_enterprises",
			Description: `查询企业列表。参数：name/industry/scale/address/created_from/created_to/
group_by(industry,scale)/group_by_period(day,week,month,quarter,year)/
aggregate(count)/order_by(-created_at,name)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "合肥高新区有多少家信息技术行业的中型企业？")
	if name != "query_enterprises" {
		t.Errorf("expected query_enterprises, got %s", name)
	}
	// 应该包含多个过滤条件：address 含"合肥高新"、industry=信息技术、scale=中型
	if !strings.Contains(args, "信息技术") {
		t.Errorf("expected industry filter, got %s", args)
	}
}

// TestToolSelection_SortOrder 排序
func TestToolSelection_SortOrder(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_enterprises",
			Description: `查询企业列表。参数：name/industry/scale/address/created_from/created_to/
group_by/group_by_period/aggregate/order_by(-created_at,name,-id)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "列出最新入驻的5家企业")
	if name != "query_enterprises" {
		t.Errorf("expected query_enterprises, got %s", name)
	}
	if !strings.Contains(args, "created_at") {
		t.Errorf("expected order_by=-created_at, got %s", args)
	}
}

// TestToolSelection_LimitOnly 只限制数量
func TestToolSelection_LimitOnly(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_policies",
			Description: `查询政策列表。参数：title/target_role(enterprise/carrier/both)/
department/status(draft/published/closed)/start_from/start_to/end_from/end_to/
group_by(department,target_role,status)/group_by_period/
aggregate(count)/order_by(-published_at,-created_at)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "title", Type: "string", FilterMode: "like"},
				{Name: "target_role", Type: "string", FilterMode: "exact", EnumValues: []string{"enterprise", "carrier", "both"}},
				{Name: "department", Type: "string", FilterMode: "exact"},
				{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "published", "closed"}},
				{Name: "start_date", Type: "string", FilterMode: "range"},
				{Name: "end_date", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	_, name, args := runGovQuery(t, eng, "显示已发布的最新10条政策")
	if name != "query_policies" {
		t.Errorf("expected query_policies, got %s", name)
	}
	if !strings.Contains(args, "published") {
		t.Errorf("expected status=published, got %s", args)
	}
	if !strings.Contains(args, "10") {
		t.Errorf("expected limit=10, got %s", args)
	}
}

// TestToolSelection_BarChartIntent 柱状图意图（应选对工具和 group_by）
func TestToolSelection_BarChartIntent(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)

	tools := []agenttools.Tool{
		NewGenericQueryTool(&TableConfig{
			Table: "query_incubation_records",
			Description: `查询孵化记录。参数：enterprise_id/carrier_id/
incubate_status(in_incubation/graduated)/status/start_from/start_to/
group_by(carrier_id,incubate_status,status)/group_by_period/
aggregate(count)/order_by(-created_at)/limit`,
			Aggregates: []string{"count"},
			Columns: []ColumnDef{
				{Name: "enterprise_id", Type: "integer", FilterMode: "exact"},
				{Name: "carrier_id", Type: "integer", FilterMode: "exact"},
				{Name: "incubate_status", Type: "string", FilterMode: "exact", EnumValues: []string{"in_incubation", "graduated"}},
				{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "approved", "rejected", "returned"}},
				{Name: "incubate_start", Type: "string", FilterMode: "range"},
			},
		}, db),
	}
	eng := buildEngine(t, ai, db, tools)

	names, name, args := runGovQuery(t, eng, "各个载体分别有多少在孵企业？帮我做成柱状图")
	if name != "query_incubation_records" {
		t.Errorf("expected query_incubation_records, got %s", name)
	}
	if !strings.Contains(args, "carrier_id") || !strings.Contains(args, "count") {
		t.Errorf("expected group_by=carrier_id + aggregate=count, got %s", args)
	}
	// 也可能触发 generate_report，检查是否在工具列表中
	t.Logf("all tools called: %v", names)
}
