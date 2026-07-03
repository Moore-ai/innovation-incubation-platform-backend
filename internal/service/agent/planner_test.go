package agent

import (
	"strings"
	"testing"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

func TestParsePlan_ValidSingleStep(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "search"})

	text := "Plan:\n1. search_policy() — 搜索政策\n"
	plan, err := parsePlan(text, reg)

	if err != nil {
		t.Fatalf("parsePlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Tools[0] != "search_policy" {
		t.Errorf("expected search_policy, got %q", plan.Steps[0].Tools[0])
	}
	if plan.Steps[0].Desc != "搜索政策" {
		t.Errorf("expected desc '搜索政策', got %q", plan.Steps[0].Desc)
	}
}

func TestParsePlan_MultiStep(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "search"})
	reg.Register(&mockTool{name: "query_enterprise_info", desc: "info"})
	reg.Register(&mockTool{name: "query_appeal", desc: "appeal"})

	text := "Plan:\n1. search_policy() — search\n\n2. query_enterprise_info() — info\n\n3. query_appeal() — appeal"
	plan, err := parsePlan(text, reg)

	if err != nil {
		t.Fatalf("parsePlan: %v", err)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestParsePlan_ParallelTools(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "search"})
	reg.Register(&mockTool{name: "query_enterprise_info", desc: "info"})

	text := "Plan:\n1. search_policy(), query_enterprise_info() — both together\n"
	plan, err := parsePlan(text, reg)

	if err != nil {
		t.Fatalf("parsePlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if len(plan.Steps[0].Tools) != 2 {
		t.Fatalf("expected 2 parallel tools, got %d: %v", len(plan.Steps[0].Tools), plan.Steps[0].Tools)
	}
	if plan.Steps[0].Tools[0] != "search_policy" || plan.Steps[0].Tools[1] != "query_enterprise_info" {
		t.Errorf("unexpected tools: %v", plan.Steps[0].Tools)
	}
}

func TestParsePlan_EmptyInput(t *testing.T) {
	reg := agenttools.NewToolRegistry()

	_, err := parsePlan("", reg)
	if err == nil {
		t.Error("expected error for empty text")
	}

	_, err = parsePlan("no plan here", reg)
	if err == nil {
		t.Error("expected error for text without Plan:")
	}
}

func TestParsePlan_SkipUnknownTools(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "search"})

	text := "Plan:\n1. search_policy(), nonexistent_tool() — mixed\n"
	plan, err := parsePlan(text, reg)

	if err != nil {
		t.Fatalf("parsePlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if len(plan.Steps[0].Tools) != 1 {
		t.Fatalf("expected 1 valid tool (nonexistent skipped), got %d", len(plan.Steps[0].Tools))
	}
	if plan.Steps[0].Tools[0] != "search_policy" {
		t.Errorf("expected search_policy, got %q", plan.Steps[0].Tools[0])
	}
}

func TestParsePlan_CaseInsensitivePrefix(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "search"})

	for _, prefix := range []string{"Plan:", "plan:", "PLAN:"} {
		text := prefix + "\n1. search_policy() — test\n"
		plan, err := parsePlan(text, reg)
		if err != nil {
			t.Errorf("parsePlan(%q): %v", prefix, err)
			continue
		}
		if len(plan.Steps) != 1 {
			t.Errorf("parsePlan(%q): expected 1 step", prefix)
		}
	}
}

func TestParsePlan_EmDashSeparator(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "search"})

	// Unicode em dash (U+2014)
	text := "Plan:\n1. search_policy() — search policy docs\n"
	plan, err := parsePlan(text, reg)

	if err != nil {
		t.Fatalf("parsePlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Desc != "search policy docs" {
		t.Errorf("expected desc 'search policy docs', got %q", plan.Steps[0].Desc)
	}
}

func TestParsePlan_AllUnknownTools(t *testing.T) {
	reg := agenttools.NewToolRegistry()

	text := "Plan:\n1. bad_tool_a() — step 1\n2. bad_tool_b() — step 2\n"
	_, err := parsePlan(text, reg)
	if err == nil {
		t.Error("expected error when all tools are unknown")
	}
}

func TestBuildPlanPrompt_IncludesAllTools(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{name: "search_policy", desc: "根据关键词搜索政策"})
	reg.Register(&mockTool{name: "query_appeal", desc: "查询诉求状态"})

	tools := reg.All()
	prompt := buildPlanPrompt(tools)

	if !strings.Contains(prompt, "search_policy") {
		t.Error("prompt missing search_policy")
	}
	if !strings.Contains(prompt, "query_appeal") {
		t.Error("prompt missing query_appeal")
	}
	if !strings.Contains(prompt, "Plan:") {
		t.Error("prompt missing Plan format instruction")
	}
}
