package memory

import (
	"strings"
	"testing"
	"time"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

func init() {
	tokenutil.SetEstimationMode("simple")
}

// --- mock ---

type mockWorkingRepo struct {
	// LoadMessagesPage 的回调，测试可自定义行为
	fn func(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error)
}

func (m *mockWorkingRepo) LoadMessagesPage(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
	return m.fn(sessionID, cursorID, limit)
}

// --- tests ---

func TestBuildWorkingContext_NoMessages(t *testing.T) {
	mock := &mockWorkingRepo{
		fn: func(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
			return nil, 0, false, nil
		},
	}
	wm := &WorkingMemory{repo: mock, pageSize: 10}

	result, err := wm.BuildWorkingContext(1, 1000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestBuildWorkingContext_BudgetStop(t *testing.T) {
	mock := &mockWorkingRepo{
		fn: func(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
			return []model.ChatMessage{
				{BaseModel: model.BaseModel{CreatedAt: time.Now()}, Role: "user", Content: "hello"},
			}, 0, false, nil
		},
	}
	wm := &WorkingMemory{repo: mock, pageSize: 10}

	// 预算足够容纳 "user: hello\n" (简单模式 ~2 tokens)
	result, err := wm.BuildWorkingContext(1, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("expected message in result, got %q", result)
	}
}

func TestBuildWorkingContext_SkipsMessagesOverBudget(t *testing.T) {
	mock := &mockWorkingRepo{
		fn: func(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
			return []model.ChatMessage{
				{BaseModel: model.BaseModel{CreatedAt: time.Now()}, Role: "user", Content: "hi"},
				{BaseModel: model.BaseModel{CreatedAt: time.Now()}, Role: "assistant", Content: "hello world long reply"},
			}, 0, false, nil
		},
	}
	wm := &WorkingMemory{repo: mock, pageSize: 10}

	// 预算只能放一条消息
	// "user: hi\n" = 简单模式: CJK=0, fields=["user:","hi"] → 2 tokens
	// "assistant: hello world long reply\n" → fields=["assistant:","hello","world","long","reply"] → 5 tokens
	shortTokens := tokenutil.Estimate("user: hi\n")
	result, err := wm.BuildWorkingContext(1, shortTokens, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hi") {
		t.Errorf("expected 'hi' in result, got %q", result)
	}
	if strings.Contains(result, "hello world long reply") {
		t.Errorf("long message should be skipped due to budget")
	}
}

func TestBuildWorkingContext_OrderAscending(t *testing.T) {
	now := time.Now()
	mock := &mockWorkingRepo{
		fn: func(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
			// Repo 返回降序（DESC）
			return []model.ChatMessage{
				{BaseModel: model.BaseModel{CreatedAt: now.Add(3 * time.Minute)}, Role: "assistant", Content: "third"},
				{BaseModel: model.BaseModel{CreatedAt: now.Add(2 * time.Minute)}, Role: "user", Content: "second"},
				{BaseModel: model.BaseModel{CreatedAt: now.Add(1 * time.Minute)}, Role: "user", Content: "first"},
			}, 0, false, nil
		},
	}
	wm := &WorkingMemory{repo: mock, pageSize: 10}

	result, err := wm.BuildWorkingContext(1, 1000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// buildOrdered 应反转回时间升序
	firstIdx := strings.Index(result, "first")
	secondIdx := strings.Index(result, "second")
	thirdIdx := strings.Index(result, "third")
	if firstIdx < 0 || secondIdx < 0 || thirdIdx < 0 {
		t.Fatalf("missing messages in result: %q", result)
	}
	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Errorf("messages not in ascending order: %q", result)
	}
}

func TestBuildWorkingContext_MultiPage(t *testing.T) {
	callCount := 0
	mock := &mockWorkingRepo{
		fn: func(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
			callCount++
			if callCount == 1 {
				// 第一页返回满页，提示还有更多
				msgs := make([]model.ChatMessage, limit)
				for i := range limit {
					msgs[i] = model.ChatMessage{
						BaseModel: model.BaseModel{CreatedAt: time.Now()},
						Role:      "user", Content: "msg",
					}
				}
				return msgs, msgs[len(msgs)-1].ID, true, nil
			}
			// 第二页返回空
			return nil, 0, false, nil
		},
	}
	wm := &WorkingMemory{repo: mock, pageSize: 3}

	_, err := wm.BuildWorkingContext(1, 1000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 page loads, got %d", callCount)
	}
}
