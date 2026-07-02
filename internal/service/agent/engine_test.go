package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"innovation-incubation-platform-backend/config"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

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
			name: "test_tool",
			desc: "a short description",
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
		name: "echo",
		desc: "echo tool",
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
	eng := NewEngine(nil, reg, nil, nil, config.AgentConfig{ToolTimeoutSec: 5})

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
	eng := NewEngine(nil, reg, nil, nil, config.AgentConfig{ToolTimeoutSec: 5})

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
	eng := NewEngine(nil, reg, nil, checker, config.AgentConfig{})

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
	eng := NewEngine(nil, reg, nil, checker, config.AgentConfig{})

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

func TestNewEngine_RoleToolTokens(t *testing.T) {
	tool := &mockTool{
		name: "test", desc: "test tool", allowedRoles: []string{"enterprise"},
		inputSchema: json.RawMessage(`{"type":"object"}`), outputSchema: json.RawMessage(`{"type":"object"}`),
		execFn: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) { return args, nil },
	}

	reg := agenttools.NewToolRegistry()
	reg.Register(tool)
	eng := NewEngine(nil, reg, nil, nil, config.AgentConfig{
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
