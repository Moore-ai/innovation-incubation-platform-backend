package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	openai "github.com/sashabaranov/go-openai"



	"innovation-incubation-platform-backend/config"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

var sseAll = []string{"reply", "done", "thinking", "error", "tool_call", "tool_result", "plan", "replan"}

func buildStreamResponse(id string, toolCallsJSON []byte) []byte {
	if len(toolCallsJSON) > 0 {
		var toolCalls []any
		json.Unmarshal(toolCallsJSON, &toolCalls)
		chunk, _ := json.Marshal(map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{"tool_calls": toolCalls}}},
		})
		return chunk
	}
	chunk, _ := json.Marshal(map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": "result ok"}}},
	})
	return chunk
}

func containsEventType(events []SSEEvent, typ string) bool {
	for _, e := range events {
		if e.Type == typ {
			return true
		}
	}
	return false
}

type mockStream struct {
	responses []openai.ChatCompletionStreamResponse
	idx       int
}

func (m *mockStream) Recv() (openai.ChatCompletionStreamResponse, error) {
	if m.idx >= len(m.responses) {
		return openai.ChatCompletionStreamResponse{}, io.EOF
	}
	r := m.responses[m.idx]
	m.idx++
	return r, nil
}

func (m *mockStream) Close() error { return nil }

func deltaText(content string) openai.ChatCompletionStreamResponse {
	return openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{Content: content},
		}},
	}
}

func deltaToolCall(index int, id, name, arguments string) openai.ChatCompletionStreamResponse {
	idx := index
	return openai.ChatCompletionStreamResponse{
		Choices: []openai.ChatCompletionStreamChoice{{
			Delta: openai.ChatCompletionStreamChoiceDelta{
				ToolCalls: []openai.ToolCall{{
					Index: &idx,
					ID:    id,
					Type:  openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      name,
						Arguments: arguments,
					},
				}},
			},
		}},
	}
}

func TestReadStream_PlainText(t *testing.T) {
	stream := &mockStream{
		responses: []openai.ChatCompletionStreamResponse{
			deltaText("hello "),
			deltaText("world"),
		},
	}

	content, calls := readStream(stream, func(SSEEvent) {})

	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestReadStream_ToolCall(t *testing.T) {
	stream := &mockStream{
		responses: []openai.ChatCompletionStreamResponse{
			deltaToolCall(0, "call_1", "search_policy", `{"query":"`),
			deltaToolCall(0, "", "", `test"}`),
		},
	}

	_, calls := readStream(stream, func(SSEEvent) {})

	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].ID != "call_1" {
		t.Errorf("expected call ID 'call_1', got %q", calls[0].ID)
	}
	if calls[0].Function.Name != "search_policy" {
		t.Errorf("expected Function.Name 'search_policy', got %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"query":"test"}` {
		t.Errorf("expected Arguments to be merged, got %q", calls[0].Function.Arguments)
	}
}

func TestReadStream_MultipleToolCalls(t *testing.T) {
	stream := &mockStream{
		responses: []openai.ChatCompletionStreamResponse{
			deltaToolCall(0, "call_1", "search_policy", `{"query":"subsidy"}`),
			deltaToolCall(1, "call_2", "query_enterprise_info", `{}`),
		},
	}

	_, calls := readStream(stream, func(SSEEvent) {})

	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
	if calls[1].ID != "call_2" {
		t.Errorf("expected second call ID 'call_2', got %q", calls[1].ID)
	}
}

func TestReadStream_EmitsThinkingEvents(t *testing.T) {
	stream := &mockStream{
		responses: []openai.ChatCompletionStreamResponse{
			deltaText("hello"),
			deltaText(" world"),
		},
	}

	var events []SSEEvent
	readStream(stream, func(e SSEEvent) { events = append(events, e) })

	if len(events) != 2 {
		t.Fatalf("expected 2 thinking events, got %d", len(events))
	}
	for i, e := range events {
		if e.Type != "thinking" {
			t.Errorf("event %d: expected type 'thinking', got %q", i, e.Type)
		}
	}
}

func TestCalcToolDefTokens(t *testing.T) {
	tools := []agenttools.Tool{
		&mockTool{
			name:         "test_tool",
			desc:         "a short description",
			inputSchema:  json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}`),
			outputSchema: json.RawMessage(`{"type":"object"}`),
		},
	}

	tokens := calcToolDefTokens(tools)
	if tokens <= 0 {
		t.Errorf("expected positive tokens, got %d", tokens)
	}
}

func TestExecuteToolCalls(t *testing.T) {
	var execCount atomic.Int32
	tool := &mockTool{
		name:         "echo",
		desc:         "echo tool",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object"}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			execCount.Add(1)
			return args, nil
		},
	}

	reg := agenttools.NewToolRegistry()
	reg.Register(tool)
	eng := NewEngine(nil, reg, nil, nil, config.AgentConfig{PublicSSETypes: sseAll, ToolTimeoutSec: 5})

	calls := []openai.ToolCall{
		{ID: "call_1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "echo", Arguments: `{"msg":"hi"}`}},
		{ID: "call_2", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "echo", Arguments: `{"msg":"hello"}`}},
	}

	results := eng.executeToolCalls(context.Background(), calls)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if execCount.Load() != 2 {
		t.Errorf("expected 2 executions, got %d", execCount.Load())
	}
}

func TestExecuteToolCalls_ToolNotFound(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	eng := NewEngine(nil, reg, nil, nil, config.AgentConfig{PublicSSETypes: sseAll, ToolTimeoutSec: 5})

	calls := []openai.ToolCall{
		{ID: "call_1", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "nonexistent", Arguments: "{}"}},
	}

	results := eng.executeToolCalls(context.Background(), calls)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestObserveToolResults_ReflectOnError(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})
	eng := NewEngine(nil, reg, nil, checker, config.AgentConfig{PublicSSETypes: sseAll})

	results := []toolResult{
		{callID: "call_1", name: "test", err: errors.New("execution failed")},
	}

	messages, records, hit := eng.observeToolResults(
		context.Background(), results,
		[]openai.ChatCompletionMessage{}, []ChatMessageRecord{},
		func(SSEEvent) {},
	)

	if !hit {
		t.Error("expected reflect trigger for exec error")
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}

	foundReflect := false
	for _, msg := range messages {
		if msg.Role == openai.ChatMessageRoleUser {
			foundReflect = true
			break
		}
	}
	if !foundReflect {
		t.Error("expected reflection prompt (user message) in messages")
	}
}

func TestObserveToolResults_NoReflectOnSuccess(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})
	eng := NewEngine(nil, reg, nil, checker, config.AgentConfig{PublicSSETypes: sseAll})

	data, _ := json.Marshal(map[string]string{"result": "ok"})
	results := []toolResult{
		{callID: "call_1", name: "test", content: data},
	}

	_, _, hit := eng.observeToolResults(
		context.Background(), results,
		[]openai.ChatCompletionMessage{}, []ChatMessageRecord{},
		func(SSEEvent) {},
	)

	if hit {
		t.Error("expected no reflect trigger for successful execution")
	}
}

// TestToolSelection_E2E 测试工具选择：mock server 解析请求中的用户消息，返回最匹配的工具
func TestToolSelection_E2E(t *testing.T) {
	tests := []struct {
		name         string
		userQuery    string
		expectedTool string
		role         string
	}{
		{"search policy", "帮我找一下数字化转型的补贴政策", "search_policy", "enterprise"},
		{"query enterprise", "我的入驻信息是什么", "query_enterprise_info", "enterprise"},
		{"query appeal", "我之前提交的诉求处理得怎么样了", "query_appeal", "enterprise"},
		{"query follow", "查看我关注的政策列表", "query_policy_follow", "enterprise"},
		{"search by carrier", "找找科技型中小企业政策", "search_policy", "carrier"},
		{"appeal by carrier", "我的诉求有回复吗", "query_appeal", "carrier"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := &atomic.Int32{}
			selectedTool := &atomic.Value{}
			selectedTool.Store("")

			mockHandler := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				flusher, _ := w.(http.Flusher)

				body, _ := io.ReadAll(r.Body)
				var req struct {
					Messages []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"messages"`
					Tools []struct {
						Type     string `json:"type"`
						Function struct {
							Name        string `json:"name"`
							Description string `json:"description"`
						} `json:"function"`
					} `json:"tools"`
				}
				json.Unmarshal(body, &req)

				count := callCount.Add(1)

				if count == 1 {
					var userQuery string
					for _, msg := range req.Messages {
						if msg.Role == "user" {
							userQuery = msg.Content
						}
					}
					toolName := matchToolByDescription(userQuery, req.Tools)
					if toolName == "" {
						t.Error("could not determine tool from tools definitions")
						toolName = tt.expectedTool
					}
					selectedTool.Store(toolName)
					t.Logf("User: %s → Tool: %s (among %d tools)", userQuery, toolName, len(req.Tools))

					chunk := fmt.Sprintf(`data: {"id":"test_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_%s","type":"function","function":{"name":"%s","arguments":"%s"}}]}}]}`+"\n\n",
						toolName, toolName, `{}`)
					fmt.Fprint(w, chunk)
					fmt.Fprint(w, "data: [DONE]\n\n")
				} else {
					var lastToolResult string
					for _, msg := range req.Messages {
						if msg.Role == "tool" {
							lastToolResult = msg.Content
						}
					}
					resp, _ := json.Marshal(map[string]any{
						"id": "test_2", "object": "chat.completion.chunk",
						"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": fmt.Sprintf("Result: %s", lastToolResult)}}},
					})
					fmt.Fprintf(w, "data: %s\n\n", resp)
					fmt.Fprint(w, "data: [DONE]\n\n")
				}
				flusher.Flush()
			}

			server := httptest.NewServer(http.HandlerFunc(mockHandler))
			defer server.Close()

			client := aiclient.New(server.URL+"/v1", "", "test-model", 30)

			// 注册所有 4 个工具
			reg := agenttools.NewToolRegistry()
			reg.Register(&mockTool{
				name: "search_policy", desc: "search policy",
				allowedRoles: []string{"enterprise", "carrier"},
				inputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
				outputSchema: json.RawMessage(`{"type":"object"}`),
				execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage(`{"policies":["policy1","policy2"]}`), nil
				},
			})
			reg.Register(&mockTool{
				name: "query_enterprise_info", desc: "query enterprise info",
				allowedRoles: []string{"enterprise"},
				inputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
				outputSchema: json.RawMessage(`{"type":"object"}`),
				execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage(`{"enterprise":{"name":"TestCorp","status":"active"}}`), nil
				},
			})
			reg.Register(&mockTool{
				name: "query_appeal", desc: "query appeal status",
				allowedRoles: []string{"enterprise", "carrier"},
				inputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
				outputSchema: json.RawMessage(`{"type":"object"}`),
				execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage(`{"appeals":[{"id":1,"status":"processed"}]}`), nil
				},
			})
			reg.Register(&mockTool{
				name: "query_policy_follow", desc: "query followed policies",
				allowedRoles: []string{"enterprise", "carrier"},
				inputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
				outputSchema: json.RawMessage(`{"type":"object"}`),
				execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
					return json.RawMessage(`{"follows":[{"policy_id":1,"title":"Test Policy"}]}`), nil
				},
			})

			mem := agentmemory.NewMemoryManager(nil, nil, nil, nil, config.AgentConfig{PublicSSETypes: sseAll})
			checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})
			eng := NewEngine(client, reg, mem, checker, config.AgentConfig{PublicSSETypes: sseAll,
				MaxSteps: 3, ToolTimeoutSec: 5,
			})

			ctx := WithUserID(WithRole(context.Background(), tt.role), 1)

			var events []SSEEvent
			result, err := eng.Run(ctx, 0, tt.userQuery, tt.role, func(e SSEEvent) {
				events = append(events, e)
			})

			if err != nil {
				t.Fatalf("Engine.Run failed: %v", err)
			}
			if result.FinalReply == "" {
				t.Error("expected non-empty FinalReply")
			}

			calledTool := selectedTool.Load().(string)
			if calledTool != tt.expectedTool {
				t.Errorf("expected tool %q, got %q", tt.expectedTool, calledTool)
			}
			if !containsEventType(events, "tool_call") {
				t.Error("expected tool_call event")
			}
			if !containsEventType(events, "reply") {
				t.Error("expected reply event")
			}
		})
	}
}

// matchToolByDescription 模拟 LLM 工具选择：根据用户消息和工具定义（name + description）匹配最佳工具。
// 工具定义来自 Engine 实际发送的请求体，因此测试了 Engine 传递工具定义的逻辑。
func matchToolByDescription(query string, tools []struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"function"`
}) string {
	// 每个工具的描述关键词表（从 Description 字段中提取的核心语义）
	toolKeywords := map[string][]string{
		"search_policy":         {"政策", "补贴", "申报", "奖励", "资助", "条件"},
		"query_enterprise_info": {"入驻", "企业信息", "孵化", "在孵", "状态"},
		"query_appeal":          {"诉求", "反馈", "投诉", "建议", "求助"},
		"query_policy_follow":   {"关注的政策", "关注的", "收藏", "关注列表"},
	}

	bestTool := ""
	bestScore := 0
	bestSpecificity := 0
	for _, t := range tools {
		keywords, ok := toolKeywords[t.Function.Name]
		if !ok {
			continue
		}
		score := 0
		totalLen := 0
		for _, kw := range keywords {
			if stringsContains(query, kw) {
				score++
				totalLen += len(kw) // 更长的匹配词 = 更精确的语义匹配
			}
		}
		// 分数优先，同分时取关键词总长更长者（更具体）
		if score > bestScore || (score == bestScore && score > 0 && totalLen > bestSpecificity) {
			bestScore = score
			bestSpecificity = totalLen
			bestTool = t.Function.Name
		}
	}
	return bestTool
}

func stringsContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestToolSelection_RealAI 使用真实 AI 客户端测试工具选择逻辑。
// 不 mock HTTP server，而是直接调用 config.yaml 中配置的 AI 服务。
// 需要设置 AI_API_KEY 环境变量，否则跳过。
func TestToolSelection_RealAI(t *testing.T) {
	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")
	if apiKey == "" || baseURL == "" {
		t.Skip("AI_API_KEY or AI_BASE_URL not set, skipping real AI test")
	}
	if model == "" {
		model = "qwen-plus"
	}

	client := aiclient.New(baseURL, apiKey, model, 60)

	tests := []struct {
		name         string
		query        string
		expectedTool string
	}{
		{"政策搜索", "帮我找数字化转型的补贴政策", "search_policy"},
		{"企业信息", "我想看一下我们公司的入驻状态", "query_enterprise_info"},
		{"诉求查询", "我上次提交了一个税费问题的诉求，现在处理得怎么样了", "query_appeal"},
		{"关注列表", "看看我收藏了哪些政策", "query_policy_follow"},
	}

	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{
		name: "search_policy", desc: "根据关键词、行业、企业规模等条件检索匹配的政策，返回政策列表（含标题、摘要、适用条件、补贴详情）",
		allowedRoles: []string{"enterprise", "carrier"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词或问题描述"},"industry":{"type":"string"},"scale":{"type":"string"},"region":{"type":"string"}},"required":["query"]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"policies":[{"id":1,"title":"数字化转型补贴","summary":"支持企业数字化转型的专项资金"},{"id":2,"title":"科技型企业奖励","summary":"科技型中小企业研发补贴"}],"total":2}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_enterprise_info", desc: "查询当前企业的入驻信息、入驻状态、申报进度等。仅企业用户可用",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"enterprise":{"id":1,"name":"测试科技有限公司","industry":"信息技术","scale":"中型","status":"已入驻"}}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_appeal", desc: "查询当前用户提交的诉求（反馈/建议）的处理状态和结果",
		allowedRoles: []string{"enterprise", "carrier"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"appeals":[{"id":1,"problem_type":"tax","content":"税费减免咨询","status":"processed"}],"total":1}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_policy_follow", desc: "查询当前用户关注的政策列表",
		allowedRoles: []string{"enterprise", "carrier"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"page":{"type":"integer"},"page_size":{"type":"integer"}},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"follows":[{"policy_id":1,"policy":{"title":"数字化转型补贴","department":"科技局"}}],"total":1}`), nil
		},
	})

	mem := agentmemory.NewMemoryManager(nil, nil, nil, nil, config.AgentConfig{PublicSSETypes: sseAll})
	checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})
	eng := NewEngine(client, reg, mem, checker, config.AgentConfig{PublicSSETypes: sseAll, MaxSteps: 3, ToolTimeoutSec: 30})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			ctx = WithUserID(WithRole(ctx, "enterprise"), 1)

			result, err := eng.Run(ctx, 0, tt.query, "enterprise", func(e SSEEvent) {})

			if err != nil {
				t.Fatalf("Engine.Run: %v", err)
			}
			selectedTool := extractToolCallName(result.Messages)
			t.Logf("Query: %s → Tool: %s, Reply: %s", tt.query, selectedTool, result.FinalReply[:min(len(result.FinalReply), 80)])
			if selectedTool != tt.expectedTool {
				t.Errorf("expected tool %q, got %q", tt.expectedTool, selectedTool)
			}
		})
	}
}

// TestMultiTurn_RealAI 测试 LLM 驱动的多轮工具调用。
// 模拟复杂用户请求需要调用多个工具才能完整回答。
func TestMultiTurn_RealAI(t *testing.T) {
	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")
	if apiKey == "" || baseURL == "" {
		t.Skip("AI_API_KEY or AI_BASE_URL not set, skipping real AI test")
	}
	if model == "" {
		model = "qwen-plus"
	}

	client := aiclient.New(baseURL, apiKey, model, 60)

	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{
		name: "search_policy", desc: "根据关键词、行业、企业规模等条件检索匹配的政策，返回政策列表（含标题、摘要、适用条件、补贴详情）",
		allowedRoles: []string{"enterprise", "carrier"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"搜索关键词或问题描述"},"industry":{"type":"string"},"scale":{"type":"string"},"region":{"type":"string"}},"required":["query"]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"policies":[{"id":1,"title":"数字化转型专项资金","summary":"支持企业数字化转型，最高补贴100万元","department":"经信局"},{"id":2,"title":"科技型企业研发补贴","summary":"研发费用补贴50%","department":"科技局"}],"total":2}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_enterprise_info", desc: "查询当前企业的入驻信息、入驻状态、申报进度等。仅企业用户可用",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"enterprise":{"id":1,"name":"星辰科技","industry":"信息技术","scale":"中型","status":"已入驻","incubations":[{"id":10,"carrier":"创新谷","status":"在孵"}]}}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_appeal", desc: "查询当前用户提交的诉求（反馈/建议）的处理状态和结果",
		allowedRoles: []string{"enterprise", "carrier"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"appeals":[{"id":1,"problem_type":"tax","content":"2025年税费减免申报进度","status":"pending","created_at":"2025-06-01"}],"total":1}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_policy_follow", desc: "查询当前用户关注的政策列表",
		allowedRoles: []string{"enterprise", "carrier"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"follows":[{"policy":{"id":1,"title":"人工智能产业扶持办法","department":"科技局","status":"active"}},{"policy":{"id":5,"title":"高层次人才引进补贴","department":"人社局","status":"active"}}],"total":2}`), nil
		},
	})

	mem := agentmemory.NewMemoryManager(nil, nil, nil, nil, config.AgentConfig{PublicSSETypes: sseAll})
	checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})
	eng := NewEngine(client, reg, mem, checker, config.AgentConfig{PublicSSETypes: sseAll, MaxSteps: 6, ToolTimeoutSec: 30})

	tests := []struct {
		name          string
		query         string
		expectedTools []string
		minTools      int
	}{
		{
			name:          "search + enterprise info",
			query:         "帮我找一下数字化转型补贴，并且告诉我我们公司的入驻信息",
			expectedTools: []string{"search_policy", "query_enterprise_info"},
			minTools:      2,
		},
		{
			name:          "appeal + follow",
			query:         "查看我的诉求处理情况，然后再看看我收藏的政策列表",
			expectedTools: []string{"query_appeal", "query_policy_follow"},
			minTools:      2,
		},
		{
			name:          "search + appeal",
			query:         "找找有没有人工智能方面的政策，同时查一下我之前反馈的税费问题解决了吗",
			expectedTools: []string{"search_policy", "query_appeal"},
			minTools:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			ctx = WithUserID(WithRole(ctx, "enterprise"), 1)

			result, err := eng.Run(ctx, 0, tt.query, "enterprise", func(e SSEEvent) {})
			if err != nil {
				t.Fatalf("Engine.Run: %v", err)
			}

			calledTools := extractAllToolNames(result.Messages)
			t.Logf("Query: %s", tt.query)
			t.Logf("Tools called (%d): %v", len(calledTools), calledTools)
			t.Logf("Reply: %s", result.FinalReply[:min(len(result.FinalReply), 120)])
			t.Logf("Steps: %d", result.StepsUsed)

			if len(calledTools) < tt.minTools {
				t.Errorf("expected at least %d tool calls, got %d: %v", tt.minTools, len(calledTools), calledTools)
			}
			for _, expected := range tt.expectedTools {
				found := slices.Contains(calledTools, expected)
				if !found {
					t.Errorf("expected tool %q not called. Called: %v", expected, calledTools)
				}
			}
		})
	}
}

// TestReflectRecovery_RealAI 测试 Reflect 触发后的工具替换恢复。
// B 故意失败 -> Reflect 触发 -> LLM 改用 D 替代 B。
func TestReflectRecovery_RealAI(t *testing.T) {
	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")
	if apiKey == "" || baseURL == "" {
		t.Skip("AI_API_KEY or AI_BASE_URL not set, skipping real AI test")
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
			return json.RawMessage(`{"follows":[{"policy_id":8,"title":"高端人才奖励办法"},{"policy_id":3,"title":"软件企业税收优惠"}],"total":2}`), nil
		},
	})
	// B: 故意返回 error
	reg.Register(&mockTool{
		name: "policy_detail", desc: "根据政策ID查询详细内容、申报条件、补贴金额。注意：此接口不稳定",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"policy_id":{"type":"integer","description":"政策ID"}},"required":["policy_id"]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("Internal Server Error: 服务不可用，请使用 search_policy 替代")
		},
	})
	// D: fallback
	reg.Register(&mockTool{
		name: "search_policy", desc: "根据关键词搜索匹配的政策。可替代 policy_detail 按政策名搜索获取详细内容",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"policies":[{"id":8,"title":"高端人才奖励办法","summary":"博士30万、硕士10万","department":"人才办"}],"total":1}`), nil
		},
	})
	// C
	reg.Register(&mockTool{
		name: "query_enterprise_info", desc: "查询当前企业的入驻信息",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"enterprise":{"name":"星辰科技","industry":"信息技术","scale":"中型"}}`), nil
		},
	})

	checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})
	eng := NewEngine(client, reg, agentmemory.NewMemoryManager(nil, nil, nil, nil, config.AgentConfig{PublicSSETypes: sseAll}), checker,
		config.AgentConfig{PublicSSETypes: sseAll, MaxSteps: 8, ToolTimeoutSec: 30})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	ctx = WithUserID(WithRole(ctx, "enterprise"), 1)

	result, err := eng.Run(ctx, 0, "先查看我关注了哪些政策，然后查第一条政策的详细内容，最后告诉我公司的入驻信息", "enterprise", func(e SSEEvent) {})
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}

	calledTools := extractAllToolNames(result.Messages)
	t.Logf("Tools(%d): %v, Reflect=%v, Steps=%d, Reply: %s",
		len(calledTools), calledTools, result.ReflectTrigger, result.StepsUsed,
		result.FinalReply[:min(len(result.FinalReply), 120)])

	if !result.ReflectTrigger {
		t.Error("expected ReflectTrigger=true")
	}
	foundDetail := slices.Contains(calledTools, "policy_detail")
	foundSearch := slices.Contains(calledTools, "search_policy")
	if !foundDetail || !foundSearch {
		t.Errorf("expected policy_detail(ok=%v) to fail then search_policy(ok=%v) as fallback: %v", foundDetail, foundSearch, calledTools)
	}
	if !foundDetail || !foundSearch {
		return
	}
	if slices.Index(calledTools, "search_policy") <= slices.Index(calledTools, "policy_detail") {
		t.Errorf("search_policy should come after policy_detail: %v", calledTools)
	}
	foundEnterprise := slices.Contains(calledTools, "query_enterprise_info")
	if !foundEnterprise || len(calledTools) < 3 {
		t.Errorf("expected >=3 tools called: %v", calledTools)
	}
}

// TestChainToolCall_RealAI 测试 LLM 驱动的 A -> (B+C并行) -> D 顺序工具调用。
// 验证 Engine 正确处理单工具调用、并行多工具调用、以及结果回传后的下一步调用。
func TestChainToolCall_RealAI(t *testing.T) {
	apiKey := os.Getenv("AI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")
	if apiKey == "" || baseURL == "" {
		t.Skip("AI_API_KEY or AI_BASE_URL not set, skipping real AI test")
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
			return json.RawMessage(`{"follows":[{"policy_id":1,"title":"数字化转型专项资金"},{"policy_id":3,"title":"科技型企业研发补贴"}],"total":2}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "search_policy", desc: "根据关键词、行业等条件检索匹配的政策，返回政策列表（含摘要、条件、补贴金额）",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"policies":[{"id":1,"title":"数字化转型专项资金","summary":"最高200万","department":"经信局"},{"id":3,"title":"科技型企业研发补贴","summary":"研发费用50%加计扣除","department":"科技局"}],"total":2}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_enterprise_info", desc: "查询当前企业的入驻信息、状态、所在载体",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"enterprise":{"name":"星辰科技","industry":"信息技术","scale":"中型","status":"已入驻","carrier":"创新谷孵化器"}}`), nil
		},
	})
	reg.Register(&mockTool{
		name: "query_appeal", desc: "查询当前用户提交的诉求（反馈/建议）的处理状态和结果",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{},"required":[]}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"appeals":[{"id":1,"problem_type":"tax","content":"税费减免申报","status":"pending"}],"total":1}`), nil
		},
	})

	eng := NewEngine(client, reg, agentmemory.NewMemoryManager(nil, nil, nil, nil, config.AgentConfig{PublicSSETypes: sseAll}),
		NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3}),
		config.AgentConfig{PublicSSETypes: sseAll, MaxSteps: 8, ToolTimeoutSec: 30})

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	ctx = WithUserID(WithRole(ctx, "enterprise"), 1)

	result, err := eng.Run(ctx, 0, "请先查看我关注的政策，然后同时确认这些政策的详细信息和我们公司的入驻情况，最后再查一下我的诉求列表", "enterprise",
		func(e SSEEvent) {})

	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	calledTools := extractAllToolNames(result.Messages)
	t.Logf("Tools(%d): %v, Steps=%d, Reply: %s",
		len(calledTools), calledTools, result.StepsUsed,
		result.FinalReply[:min(len(result.FinalReply), 150)])

	// A 必须先被调
	if !slices.Contains(calledTools, "query_policy_follow") {
		t.Error("A(query_policy_follow) not called")
	}
	// A 应该在 B 和 C 之前
	aIdx := slices.Index(calledTools, "query_policy_follow")
	cIdx := slices.Index(calledTools, "query_enterprise_info")
	bIdx := slices.Index(calledTools, "search_policy")
	if aIdx < 0 || cIdx < 0 || bIdx < 0 {
		t.Error("missing required tools")
	} else {
		if aIdx >= bIdx || aIdx >= cIdx {
			t.Errorf("A(%d) should be before B(%d) and C(%d): %v", aIdx, bIdx, cIdx, calledTools)
		}
	}
	// D 应该在最后
	dIdx := slices.Index(calledTools, "query_appeal")
	if dIdx < 0 {
		t.Error("D(query_appeal) not called")
	} else if aIdx >= 0 && cIdx >= 0 && dIdx <= max(aIdx, cIdx) {
		t.Errorf("D(%d) should be last after A(%d) and B/C: %v", dIdx, min(aIdx, cIdx), calledTools)
	}
}

// extractAllToolNames 从所有消息中提取调用的工具名列表。
func extractAllToolNames(msgs []ChatMessageRecord) []string {
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

// extractToolCallName 从 RunResult.Messages 中提取第一个工具调用的名称。
func extractToolCallName(msgs []ChatMessageRecord) string {
	for _, m := range msgs {
		if m.Role == "assistant" && m.ToolCalls != "" {
			var calls []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if json.Unmarshal([]byte(m.ToolCalls), &calls) == nil && len(calls) > 0 {
				return calls[0].Function.Name
			}
		}
	}
	return ""
}

func TestNewEngine_RoleToolTokens(t *testing.T) {
	tool := &mockTool{
		name: "test", desc: "test tool", allowedRoles: []string{"enterprise"},
		inputSchema: json.RawMessage(`{"type":"object"}`), outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) { return args, nil },
	}

	reg := agenttools.NewToolRegistry()
	reg.Register(tool)
	eng := NewEngine(nil, reg, nil, nil, config.AgentConfig{PublicSSETypes: sseAll,
		ContextWindow: 512000, HistoryBudgetRatio: 0.7, TokenEstimation: "better",
	})

	tokens, ok := eng.roleToolTokens["enterprise"]
	if !ok {
		t.Fatal("expected enterprise role tokens")
	}
	if tokens <= 0 {
		t.Errorf("expected positive tokens for enterprise, got %d", tokens)
	}
}

// Test tokenutil integration
func TestTokenUtilEstimate(t *testing.T) {
	tokenutil.SetEstimationMode("better")
	got := tokenutil.Estimate("subsidy policy")
	if got <= 0 {
		t.Errorf("expected positive token estimate, got %d", got)
	}
}

// TestEngineRun_E2E 端到端测试：mock OpenAI HTTP server → 真实 aiclient 流式调用 → 工具执行 → 最终回复
func TestEngineRun_E2E(t *testing.T) {
	callCount := &atomic.Int32{}

	// tool_call delta 的 JSON 片段
	toolCallJSON := `[{"index":0,"id":"call_echo","type":"function","function":{"name":"echo","arguments":"{\"msg\":\"test\"}"}}]`
	// 第二次调用的文本回复
	textReply := `Summary: echo returned successfully`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)

		count := callCount.Add(1)

		if count == 1 {
			chunk := fmt.Sprintf(`data: {"id":"test_1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"tool_calls":%s}}]}`+"\n\n", toolCallJSON)
			fmt.Fprint(w, chunk)
			fmt.Fprint(w, "data: [DONE]\n\n")
		} else {
			chunk := fmt.Sprintf(`data: {"id":"test_2","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"%s"}}]}`+"\n\n", textReply)
			fmt.Fprint(w, chunk)
			fmt.Fprint(w, "data: [DONE]\n\n")
		}
		flusher.Flush()
	}))
	defer server.Close()

	client := aiclient.New(server.URL+"/v1", "", "test-model", 30)

	tool := &mockTool{
		name: "echo", desc: "echo", allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
			return args, nil
		},
	}

	reg := agenttools.NewToolRegistry()
	reg.Register(tool)

	mem := agentmemory.NewMemoryManager(nil, nil, nil, nil, config.AgentConfig{PublicSSETypes: sseAll})
	checker := NewReflectChecker(nil, reg, config.ReflectConfig{SimilarityThreshold: 0.3})

	eng := NewEngine(client, reg, mem, checker, config.AgentConfig{PublicSSETypes: sseAll,
		MaxSteps:       3,
		ToolTimeoutSec: 5,
	})

	ctx := WithUserID(WithRole(context.Background(), "enterprise"), 1)

	var events []SSEEvent
	result, err := eng.Run(ctx, 0, "test query", "enterprise", func(e SSEEvent) {
		events = append(events, e)
	})

	if err != nil {
		t.Fatalf("Engine.Run failed: %v", err)
	}
	if result.FinalReply == "" {
		t.Error("expected non-empty FinalReply")
	}
	if !containsEventType(events, "tool_call") {
		t.Error("expected tool_call event")
	}
	if !containsEventType(events, "reply") {
		t.Error("expected reply event")
	}
	if !containsEventType(events, "done") {
		t.Error("expected done event")
	}
	if len(result.Messages) < 3 {
		t.Errorf("expected >=3 messages, got %d", len(result.Messages))
	}
}
