package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

// mockTool 用于测试的通用 mock 工具
type mockTool struct {
	name         string
	desc         string
	allowedRoles []string
	inputSchema  json.RawMessage
	outputSchema json.RawMessage
	execFn       func(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}

func (m *mockTool) Name() string                        { return m.name }
func (m *mockTool) Description() string                 { return m.desc }
func (m *mockTool) AllowedRoles() []string              { return m.allowedRoles }
func (m *mockTool) InputSchema() json.RawMessage        { return m.inputSchema }
func (m *mockTool) OutputSchema() json.RawMessage       { return m.outputSchema }
func (m *mockTool) Timeout() time.Duration               { return agenttools.DefaultTimeout() }
func (m *mockTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	return m.execFn(ctx, args)
}

func TestReflectChecker_HardRules(t *testing.T) {
	reg := agenttools.NewToolRegistry()
	reg.Register(&mockTool{
		name: "test_tool",
		desc: "a test tool",
		allowedRoles: []string{"enterprise"},
		inputSchema:  json.RawMessage(`{"type":"object","properties":{}}`),
		outputSchema: json.RawMessage(`{"type":"object"}`),
	})

	checker := NewReflectChecker(reg)

	ctx := context.Background()

	t.Run("layer1_exec_error_returns_hit", func(t *testing.T) {
		hit, reason := checker.Check(ctx, "test_tool", nil, fmt.Errorf("connection failed"))
		if !hit {
			t.Error("expected hit for exec error")
		}
		if reason == "" {
			t.Error("expected non-empty reason")
		}
	})

	t.Run("layer1_empty_result_returns_hit", func(t *testing.T) {
		hit, _ := checker.Check(ctx, "test_tool", json.RawMessage(""), nil)
		if !hit {
			t.Error("expected hit for empty result")
		}
	})

	t.Run("layer1_keyword_returns_hit", func(t *testing.T) {
		data, _ := json.Marshal(map[string]string{"error": "权限不足"})
		hit, _ := checker.Check(ctx, "test_tool", data, nil)
		if !hit {
			t.Error("expected hit for error keyword '权限不足'")
		}
	})

	t.Run("layer1_normal_result_no_hit", func(t *testing.T) {
		data, _ := json.Marshal(map[string]string{"result": "success"})
		hit, reason := checker.Check(ctx, "test_tool", data, nil)
		if hit {
			t.Errorf("expected no hit for normal result, got: %s", reason)
		}
	})

	t.Run("layer2_invalid_json_returns_hit", func(t *testing.T) {
		hit, _ := checker.Check(ctx, "test_tool", json.RawMessage(`{invalid}`), nil)
		if !hit {
			t.Error("expected hit for invalid JSON")
		}
	})

	t.Run("layer2_null_result_no_error", func(t *testing.T) {
		hit, _ := checker.Check(ctx, "test_tool", json.RawMessage(`null`), nil)
		if !hit {
			t.Error("expected hit for 'null' JSON value")
		}
	})
}
