package builtin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

func TestRecordSemanticMemory_Name(t *testing.T) {
	t.Parallel()
	tool := NewRecordSemanticMemory(nil, nil)
	if got := tool.Name(); got != "record_semantic_memory" {
		t.Errorf("expected record_semantic_memory, got %s", got)
	}
}

func TestRecordSemanticMemory_InputSchema(t *testing.T) {
	t.Parallel()
	tool := NewRecordSemanticMemory(nil, nil)
	var schema map[string]any
	if err := json.Unmarshal(tool.InputSchema(), &schema); err != nil {
		t.Fatalf("invalid schema: %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties")
	}
	for _, key := range []string{"lessons", "preferences"} {
		if _, ok := props[key]; !ok {
			t.Errorf("missing property: %s", key)
		}
	}
}

func TestRecordSemanticMemory_Execute_EmptyInput(t *testing.T) {
	db, chatRepo := newTestDB(t)
	tool := NewRecordSemanticMemory(chatRepo, nil)
	ctx := context.WithValue(context.Background(), agenttools.CtxKeyUserID, uint(1))
	result, err := tool.Execute(ctx, json.RawMessage(`{"lessons":[],"preferences":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("invalid result: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success to be true")
	}
	saved, ok := resp["saved"].(float64)
	if !ok || saved != 0 {
		t.Errorf("expected saved=0, got %v", resp["saved"])
	}
	// Verify no records persisted
	var count int64
	db.Model(&model.SemanticMemory{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 records, got %d", count)
	}
}

func TestRecordSemanticMemory_Execute_SavesContent(t *testing.T) {
	db, chatRepo := newTestDB(t)
	tool := NewRecordSemanticMemory(chatRepo, nil)
	ctx := context.WithValue(context.Background(), agenttools.CtxKeyUserID, uint(1))
	result, err := tool.Execute(ctx, json.RawMessage(`{"lessons":["lesson1"],"preferences":["pref1","pref2"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("invalid result: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success to be true")
	}
	saved, ok := resp["saved"].(float64)
	if !ok || saved != 3 {
		t.Errorf("expected saved=3, got %v", resp["saved"])
	}
	// Verify persisted
	var count int64
	db.Model(&model.SemanticMemory{}).Count(&count)
	if count != 3 {
		t.Errorf("expected 3 records in DB, got %d", count)
	}
}

func TestRecordSemanticMemory_Execute_TrimsWhitespace(t *testing.T) {
	db, chatRepo := newTestDB(t)
	tool := NewRecordSemanticMemory(chatRepo, nil)
	ctx := context.WithValue(context.Background(), agenttools.CtxKeyUserID, uint(1))
	result, err := tool.Execute(ctx, json.RawMessage(`{"lessons":["  lesson1  ","\tlesson2\n"],"preferences":["  pref1  ",""]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("invalid result: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success to be true")
	}
	saved, ok := resp["saved"].(float64)
	// 2 lessons + 1 pref (empty string skipped) = 3 saved
	if !ok || saved != 3 {
		t.Errorf("expected saved=3, got %v", resp["saved"])
	}
	var count int64
	db.Model(&model.SemanticMemory{}).Count(&count)
	if count != 3 {
		t.Errorf("expected 3 records in DB, got %d", count)
	}
}

func TestRecordSemanticMemory_Execute_MissingUserID(t *testing.T) {
	_, chatRepo := newTestDB(t)
	tool := NewRecordSemanticMemory(chatRepo, nil)
	// No userID in context - tool should not panic, saves with userID=0
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"lessons":["lesson1"],"preferences":["pref1"]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("invalid result: %v", err)
	}
	if resp["success"] != true {
		t.Error("expected success to be true")
	}
	// Without userID it still saves with userID=0
	saved, ok := resp["saved"].(float64)
	if !ok || saved == 0 {
		t.Error("expected saved > 0 even without userID in context")
	}
}

func TestRecordSemanticMemory_Timeout(t *testing.T) {
	t.Parallel()
	tool := NewRecordSemanticMemory(nil, nil)
	if got := tool.Timeout(); got <= 0 {
		t.Errorf("expected positive timeout, got %v", got)
	}
}

func TestRecordSemanticMemory_AllowedRoles(t *testing.T) {
	t.Parallel()
	tool := NewRecordSemanticMemory(nil, nil)
	roles := tool.AllowedRoles()
	if len(roles) == 0 {
		t.Fatal("expected at least one role")
	}
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}
	for _, expected := range []string{"enterprise", "carrier", "government"} {
		if !roleSet[expected] {
			t.Errorf("missing role: %s", expected)
		}
	}
}

// newTestDB creates an in-memory SQLite database and migrates the SemanticMemory table.
func newTestDB(t *testing.T) (*gorm.DB, *repository.ChatRepo) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.SemanticMemory{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db, repository.NewChatRepo(db)
}
