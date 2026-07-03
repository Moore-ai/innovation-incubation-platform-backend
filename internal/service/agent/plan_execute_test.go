package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"innovation-incubation-platform-backend/config"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

func TestPlanExecute_RealAI(t *testing.T) {
	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")
	if apiKey == "" || baseURL == "" {
		t.Skip("AI_API_KEY or AI_BASE_URL not set")
	}
	if model == "" {
		model = "qwen-plus"
	}
	client := aiclient.New(baseURL, apiKey, model, 60)

	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{
		name: "query_policy_follow", desc: "查询当前用户关注的政策列表，返回政策ID和标题",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"follows":[{"policy_id":1,"title":"数字化转型奖金"},{"policy_id":2,"title":"科技企业补贴"}],"total":2}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "search_policy", desc: "根据关键词搜索政策详细信息",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"policies":[{"id":1,"title":"数字化转型奖金","summary":"最高100万","department":"经信局"}],"total":1}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_enterprise_info", desc: "查询当前企业的入驻信息、行业、规模",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"enterprise":{"name":"星辰科技","industry":"信息技术","scale":"中型"}}`), nil
		},
	})

	eng := NewEngine(client, reg, agentmemory.NewMemoryManager(nil, nil, config.AgentConfig{PublicSSETypes: sseAll}),
		NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3}),
		config.AgentConfig{
			PublicSSETypes:  sseAll,
			PlanningEnabled: true,
			MaxSteps:        8,
			ToolTimeoutSec:  30,
		})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	ctx = WithUserID(WithRole(ctx, "enterprise"), 1)

	var planEmitted, stepsCompleted bool
	var calledTools []string
	result, err := eng.Run(ctx, 0, "先查看我关注了哪些政策，然后查第一条政策的详细内容，最后告诉我公司的入驻信息", "enterprise",
		func(e SSEEvent) {
			if e.Type == "plan" {
				planEmitted = true
			}
			if e.Type == "tool_call" {
				if calls, ok := e.Data.([]openai.ToolCall); ok {
					for _, c := range calls {
						calledTools = append(calledTools, c.Function.Name)
					}
				}
			}
			if e.Type == "done" {
				stepsCompleted = true
			}
		})

	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	t.Logf("planEmitted=%v stepsCompleted=%v tools=%v final=%s",
		planEmitted, stepsCompleted, calledTools, result.FinalReply[:min(len(result.FinalReply), 120)])

	if !planEmitted {
		t.Error("expected 'plan' SSE event")
	}
	if !stepsCompleted {
		t.Error("expected 'done' SSE event")
	}
	if len(calledTools) < 3 {
		t.Errorf("expected at least 3 tool calls, got %d: %v", len(calledTools), calledTools)
	}
	if result.FinalReply == "" {
		t.Error("expected non-empty final reply")
	}
}
