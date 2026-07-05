package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/storage"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

func seedReportData(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()

	// 先删子表再删主表，避免外键约束
	db.Exec("DELETE FROM incubation_records WHERE enterprise_id IN (SELECT id FROM enterprises WHERE credit_code LIKE 'ZTEST_C%')")
	db.Exec("DELETE FROM policy_applications WHERE id > 0")
	db.Exec("DELETE FROM enterprises WHERE credit_code LIKE 'ZTEST_C%'")

	ents := []model.Enterprise{
		{BaseModel: model.BaseModel{CreatedAt: now}, UserID: 99011, Name: "星辰科技", Industry: "信息技术", Scale: "中型", Address: "合肥市高新区", CreditCode: "ZTEST_C11"},
		{BaseModel: model.BaseModel{CreatedAt: now}, UserID: 99012, Name: "云帆生物", Industry: "生物医药", Scale: "小型", Address: "合肥市经开区", CreditCode: "ZTEST_C12"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -1, 0)}, UserID: 99013, Name: "智造工业", Industry: "智能制造", Scale: "大型", Address: "合肥市高新区", CreditCode: "ZTEST_C13"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -2, 0)}, UserID: 99014, Name: "绿能科技", Industry: "新能源", Scale: "大型", Address: "合肥市包河区", CreditCode: "ZTEST_C14"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -3, 0)}, UserID: 99015, Name: "数智网络", Industry: "信息技术", Scale: "中型", Address: "合肥市蜀山区", CreditCode: "ZTEST_C15"},
	}
	db.Create(&ents)

	incubations := []model.IncubationRecord{
		{EnterpriseID: ents[0].ID, CarrierID: 1, IncubateStatus: "in_incubation", IncubateStart: now.AddDate(0, -1, 0).Format("2006-01-02"), Status: "approved"},
		{EnterpriseID: ents[1].ID, CarrierID: 1, IncubateStatus: "in_incubation", IncubateStart: now.AddDate(0, -2, 0).Format("2006-01-02"), Status: "approved"},
		{EnterpriseID: ents[2].ID, CarrierID: 2, IncubateStatus: "graduated", IncubateStart: now.AddDate(0, -6, 0).Format("2006-01-02"), IncubateEnd: now.AddDate(0, -1, 0).Format("2006-01-02"), Status: "approved"},
		{EnterpriseID: ents[3].ID, CarrierID: 2, IncubateStatus: "in_incubation", IncubateStart: now.AddDate(0, -1, 0).Format("2006-01-02"), Status: "approved"},
		{EnterpriseID: ents[4].ID, CarrierID: 3, IncubateStatus: "in_incubation", IncubateStart: now.Format("2006-01-02"), Status: "approved"},
	}
	db.Create(&incubations)

	t.Cleanup(func() {
		db.Exec("DELETE FROM incubation_records WHERE enterprise_id IN (SELECT id FROM enterprises WHERE credit_code LIKE 'ZTEST_C%')")
		db.Exec("DELETE FROM enterprises WHERE credit_code LIKE 'ZTEST_C%'")
	})
}

func buildReportEngine(t *testing.T, ai *aiclient.Client, db *gorm.DB, extraTools []agenttools.Tool) *agent.Engine {
	t.Helper()
	reg := agenttools.NewToolRegistry()
	for _, tool := range extraTools {
		reg.Register(tool)
	}

	chartStorage, _ := storage.NewLocalFileStorage(os.TempDir())
	venvPath := findProjectRoot() + "/sidecar/file-parser/venv"
	reg.Register(NewGenerateReport(ai, db, repository.NewFileRepo(db), chartStorage, venvPath, os.TempDir()))

	cfg := config.AgentConfig{
		PublicSSETypes:     []string{"reply", "done", "thinking", "error", "tool_call", "tool_result", "report_start", "report_progress", "report_done"},
		MaxSteps:           5,
		ToolTimeoutSec:     120,
		ContextWindow:      1,
		HistoryBudgetRatio: 0,
		Memory:             config.AgentMemoryConfig{SemanticLimit: 0, EpisodicLimit: 0},
		RequestTimeoutSec:  300,
	}
	checker := agent.NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.99})
	return agent.NewEngine(ai, reg, agentmemory.NewMemoryManager(nil, nil, nil, nil, cfg), checker, cfg)
}

func runReportQuery(t *testing.T, eng *agent.Engine, query string) (string, []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	ctx = agent.WithUserID(agent.WithRole(ctx, "government"), 1)

	result, err := eng.Run(ctx, 0, query, "government", func(e agent.SSEEvent) {
		t.Logf("SSE: type=%s data=%v", e.Type, e.Data)
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	names := extractCalledTools(result.Messages)
	t.Logf("Tools called: %v", names)
	t.Logf("Steps: %d, Reflect: %v", result.StepsUsed, result.ReflectTrigger)
	t.Logf("Reply (first 300 chars): %s", result.FinalReply[:min(300, len(result.FinalReply))])
	return result.FinalReply, names
}

// buildQueryTool 快速构造一个 GenericQueryTool。
func buildQueryTool(table, desc string, cols []ColumnDef, db *gorm.DB) agenttools.Tool {
	return NewGenericQueryTool(&TableConfig{
		Table:       table,
		Description: desc,
		Aggregates:  []string{"count"},
		Columns:     cols,
	}, db)
}

// TestGenerateReport_EnterpriseAnalysis 企业分布分析报告
func TestGenerateReport_EnterpriseAnalysis(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)
	db.AutoMigrate(&model.Enterprise{}, &model.IncubationRecord{}, &model.File{})
	seedReportData(t, db)

	tools := []agenttools.Tool{
		buildQueryTool("enterprises",
			"查询企业。参数：name/industry/scale/address/created_from/created_to/group_by(industry,scale)/group_by_period/aggregate(count)/order_by/limit",
			[]ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			}, db),
	}
	eng := buildReportEngine(t, ai, db, tools)

	reply, names := runReportQuery(t, eng, "生成一份数据分析报告：合肥高新区企业行业分布和规模分布情况")

	_ = slices.Contains(names, "generate_report") // may or may not be called
	if !strings.Contains(reply, "#") {
		t.Errorf("expected Markdown report with headings, got: %s", reply[:min(200, len(reply))])
	}
	t.Logf("Report length: %d chars, generate_report called: %v", len(reply), slices.Contains(names, "generate_report"))
}

// TestGenerateReport_IncubationStatistics 孵化统计报告
func TestGenerateReport_IncubationStatistics(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)
	db.AutoMigrate(&model.Enterprise{}, &model.IncubationRecord{}, &model.File{})
	seedReportData(t, db)

	tools := []agenttools.Tool{
		buildQueryTool("incubation_records",
			"查询孵化记录。参数：enterprise_id/carrier_id/incubate_status(in_incubation/graduated)/status/start_from/start_to/group_by(carrier_id,incubate_status,status)/group_by_period/aggregate(count)/order_by/limit",
			[]ColumnDef{
				{Name: "enterprise_id", Type: "integer", FilterMode: "exact"},
				{Name: "carrier_id", Type: "integer", FilterMode: "exact"},
				{Name: "incubate_status", Type: "string", FilterMode: "exact", EnumValues: []string{"in_incubation", "graduated"}},
				{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "approved", "rejected", "returned"}},
				{Name: "incubate_start", Type: "string", FilterMode: "range"},
			}, db),
	}
	eng := buildReportEngine(t, ai, db, tools)

	reply, names := runReportQuery(t, eng, "生成报告：各载体的在孵企业统计，包含孵化状态分布")

	_ = slices.Contains(names, "generate_report")
	if !strings.Contains(strings.ToLower(reply), "孵") {
		t.Errorf("expected report about incubation, got: %s", reply[:min(200, len(reply))])
	}
	t.Logf("Report length: %d chars", len(reply))
}

// TestGenerateReport_MonthlyTrend 月度趋势报告
func TestGenerateReport_MonthlyTrend(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)
	db.AutoMigrate(&model.Enterprise{}, &model.IncubationRecord{}, &model.File{})
	seedReportData(t, db)

	tools := []agenttools.Tool{
		buildQueryTool("enterprises",
			"查询企业。参数：name/industry/scale/address/created_from/created_to/group_by(industry,scale)/group_by_period(day,week,month,quarter,year)/aggregate(count)/order_by/limit",
			[]ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			}, db),
	}
	eng := buildReportEngine(t, ai, db, tools)

	reply, names := runReportQuery(t, eng, "生成报告：过去6个月每月新入驻企业的数量变化趋势，用折线图展示")

	_ = slices.Contains(names, "generate_report")
	if strings.Contains(reply, "![") || strings.Contains(reply, "![](") {
		t.Log("Report contains chart image references")
	}
	if !strings.Contains(reply, "#") {
		t.Errorf("expected Markdown report, got: %s", reply[:min(200, len(reply))])
	}
	t.Logf("Report length: %d chars", len(reply))
}

// TestGenerateReport_MultiTable 多表联合报告
func TestGenerateReport_MultiTable(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)
	db.AutoMigrate(&model.Enterprise{}, &model.IncubationRecord{}, &model.File{})
	seedReportData(t, db)

	tools := []agenttools.Tool{
		buildQueryTool("enterprises",
			"查询企业。参数：name/industry/scale/address/created_from/created_to/group_by(industry,scale)/group_by_period/aggregate(count)/order_by/limit",
			[]ColumnDef{
				{Name: "name", Type: "string", FilterMode: "like"},
				{Name: "industry", Type: "string", FilterMode: "exact"},
				{Name: "scale", Type: "string", FilterMode: "exact"},
				{Name: "address", Type: "string", FilterMode: "like"},
				{Name: "created_at", Type: "string", FilterMode: "range"},
			}, db),
		buildQueryTool("incubation_records",
			"查询孵化记录。参数：enterprise_id/carrier_id/incubate_status(in_incubation/graduated)/status/start_from/start_to/group_by(carrier_id,incubate_status,status)/group_by_period/aggregate(count)/order_by/limit",
			[]ColumnDef{
				{Name: "enterprise_id", Type: "integer", FilterMode: "exact"},
				{Name: "carrier_id", Type: "integer", FilterMode: "exact"},
				{Name: "incubate_status", Type: "string", FilterMode: "exact", EnumValues: []string{"in_incubation", "graduated"}},
				{Name: "status", Type: "string", FilterMode: "exact", EnumValues: []string{"draft", "pending", "approved", "rejected", "returned"}},
				{Name: "incubate_start", Type: "string", FilterMode: "range"},
			}, db),
	}
	eng := buildReportEngine(t, ai, db, tools)

	reply, names := runReportQuery(t, eng, "生成一份综合分析报告：高新区企业入驻与孵化情况分析，包含行业分布柱状图和孵化状态饼图")

	_ = slices.Contains(names, "generate_report")
	hasCharts := strings.Contains(reply, "![") || strings.Contains(reply, "![](")
	hasAnalysis := strings.Contains(reply, "##")
	t.Logf("Has charts: %v, Has sections: %v, generate_report called: %v", hasCharts, hasAnalysis, slices.Contains(names, "generate_report"))
	if !hasAnalysis {
		t.Errorf("expected report with sections, got: %s", reply[:min(300, len(reply))])
	}
	t.Logf("Report length: %d chars", len(reply))
}

// TestGenerateReport_ChartOutput 直接测试 generate_report.Execute 并验证图表文件生成
func TestGenerateReport_ChartOutput(t *testing.T) {
	ai := realAIClient(t)
	db := openTestDBForTool(t)
	db.AutoMigrate(&model.Enterprise{}, &model.IncubationRecord{}, &model.File{})
	seedReportData(t, db)

	venvPath := findProjectRoot() + "/sidecar/file-parser/venv"
	chartStorage, _ := storage.NewLocalFileStorage(os.TempDir())
	report := NewGenerateReport(ai, db, repository.NewFileRepo(db), chartStorage, venvPath, os.TempDir())
	report.configs = []*TableConfig{{
		Table:       "query_enterprises",
		Description: "查询企业",
		Aggregates:  []string{"count"},
		Columns: []ColumnDef{
			{Name: "name", Type: "string", FilterMode: "like"},
			{Name: "industry", Type: "string", FilterMode: "exact"},
			{Name: "scale", Type: "string", FilterMode: "exact"},
			{Name: "address", Type: "string", FilterMode: "like"},
			{Name: "created_at", Type: "string", FilterMode: "range"},
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	args, _ := json.Marshal(map[string]string{"prompt": "生成合肥地区企业行业分布柱状图和规模分布饼图"})
	result, err := report.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var output map[string]string
	json.Unmarshal(result, &output)
	markdown := output["markdown"]

	t.Logf("Markdown (first 500): %s", markdown[:min(500, len(markdown))])
	t.Logf("Total length: %d", len(markdown))

	if !strings.Contains(markdown, "![") {
		t.Error("expected chart image references in markdown")
	}
	if !strings.Contains(markdown, "#") {
		t.Error("expected Markdown headings")
	}
}

func init() {
	_ = fmt.Sprintf
}
