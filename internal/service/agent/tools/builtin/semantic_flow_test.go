package builtin

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"innovation-incubation-platform-backend/internal/model"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

// memStore 内存数组实现，替代 SQLite
type memStore struct {
	records []model.SemanticMemory
}

func newMemStore() *memStore {
	return &memStore{}
}

func (s *memStore) CreateSemanticMemory(m *model.SemanticMemory) error {
	m.ID = uint(len(s.records) + 1)
	s.records = append(s.records, *m)
	return nil
}

func (s *memStore) Records() []model.SemanticMemory {
	return slices.Clone(s.records)
}

func TestRecordSemanticMemory_FullFlow(t *testing.T) {
	store := newMemStore()
	tool := NewRecordSemanticMemory(store, nil)

	ctx := context.WithValue(context.Background(), agenttools.CtxKeyUserID, uint(42))

	// Round 1: 用户表达偏好
	_, err := tool.Execute(ctx, json.RawMessage(`{"preferences":["用户偏好：默认使用PDF格式"]}`))
	if err != nil {
		t.Fatalf("round 1 failed: %v", err)
	}

	records := store.Records()
	if len(records) != 1 {
		t.Fatalf("round 1: expected 1 record, got %d", len(records))
	}
	if records[0].Category != "preference" {
		t.Errorf("round 1: expected category=preference, got %s", records[0].Category)
	}
	if records[0].Content != "用户偏好：默认使用PDF格式" {
		t.Errorf("round 1: content mismatch: %s", records[0].Content)
	}

	// Round 2: 另一条偏好
	_, err = tool.Execute(ctx, json.RawMessage(`{"preferences":["用户偏好：图表默认使用柱状图"]}`))
	if err != nil {
		t.Fatalf("round 2 failed: %v", err)
	}

	records = store.Records()
	if len(records) != 2 {
		t.Fatalf("round 2: expected 2 records, got %d", len(records))
	}

	// Round 3: 记录教训（工具调用失败）
	_, err = tool.Execute(ctx, json.RawMessage(`{"lessons":["教训：query_enterprise_info 缺少企业名，应提醒用户提供"]}`))
	if err != nil {
		t.Fatalf("round 3 failed: %v", err)
	}

	records = store.Records()
	if len(records) != 3 {
		t.Fatalf("round 3: expected 3 records, got %d", len(records))
	}

	// 验证教训
	lesson := records[2]
	if lesson.Category != "lesson" {
		t.Errorf("expected category=lesson, got %s", lesson.Category)
	}
	if lesson.Content != "教训：query_enterprise_info 缺少企业名，应提醒用户提供" {
		t.Errorf("lesson content mismatch: %s", lesson.Content)
	}

	// 验证 UserID 正确
	for _, r := range records {
		if r.UserID == nil || *r.UserID != 42 {
			t.Errorf("expected userID=42, got %v", r.UserID)
		}
	}

	// Round 4: 同时写入偏好+教训
	_, err = tool.Execute(ctx, json.RawMessage(`{"lessons":["教训2：xxx"],"preferences":["偏好2：yyy"]}`))
	if err != nil {
		t.Fatalf("round 4 failed: %v", err)
	}

	records = store.Records()
	if len(records) != 5 {
		t.Fatalf("round 4: expected 5 records, got %d", len(records))
	}
}
