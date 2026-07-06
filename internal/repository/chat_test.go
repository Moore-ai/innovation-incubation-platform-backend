package repository

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/database"

	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbCfg := loadDBConfig(t)
	db, err := database.NewDB(dbCfg)
	if err != nil {
		t.Skipf("跳过集成测试：无法连接数据库 (%v)", err)
	}
	db.AutoMigrate(&model.ChatSession{}, &model.ChatMessage{})
	return db
}

func loadDBConfig(t *testing.T) config.DBConfig {
	t.Helper()
	loadDotEnvForTest("../../.env")
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	if port == 0 {
		port = 5432
	}
	return config.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		SSLMode:  "disable",
		LogLevel: "warn",
	}
}

func loadDotEnvForTest(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func createTestSession(t *testing.T, db *gorm.DB) *model.ChatSession {
	t.Helper()
	sess := &model.ChatSession{
		UserID:        9999,
		Title:         "test-edit-resend",
		LastMessageAt: time.Now(),
	}
	if err := db.Create(sess).Error; err != nil {
		t.Fatalf("创建测试会话失败: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Where("id = ?", sess.ID).Delete(&model.ChatSession{})
		db.Unscoped().Where("session_id = ?", sess.ID).Delete(&model.ChatMessage{})
	})
	return sess
}

func TestReplaceMessages_SoftDeleteAndInsert(t *testing.T) {
	db := setupTestDB(t)
	repo := NewChatRepo(db)
	sess := createTestSession(t, db)

	// 种子: 3 条旧消息
	oldMsgs := []model.ChatMessage{
		{SessionID: sess.ID, UserID: 9999, Role: "user", Content: "old-q", ToolCalls: "[]"},
		{SessionID: sess.ID, UserID: 9999, Role: "assistant", Content: "old-a", ToolCalls: "[]"},
		{SessionID: sess.ID, UserID: 9999, Role: "tool", Content: "old-t", ToolCalls: "[]"},
	}
	db.Create(&oldMsgs)
	db.Model(sess).Update("message_count", 3) // 种子数据计入统计
	fromID := oldMsgs[0].ID

	// 新消息
	newMsgs := []model.ChatMessage{
		{SessionID: sess.ID, UserID: 9999, Role: "user", Content: "new-q", ToolCalls: "[]"},
		{SessionID: sess.ID, UserID: 9999, Role: "assistant", Content: "new-a", ToolCalls: "[]"},
	}

	// 执行
	deletedCount, err := repo.ReplaceMessages(sess.ID, fromID, newMsgs)
	if err != nil {
		t.Fatalf("ReplaceMessages 失败: %v", err)
	}

	if deletedCount != 3 {
		t.Errorf("期望删除 3 条，实际 %d", deletedCount)
	}

	// 验证旧消息已被软删除
	var softDeleted int64
	db.Unscoped().Model(&model.ChatMessage{}).
		Where("session_id = ? AND id >= ? AND deleted_at IS NOT NULL", sess.ID, fromID).
		Count(&softDeleted)
	if softDeleted != 3 {
		t.Errorf("期望 3 条软删记录，实际 %d", softDeleted)
	}

	// 验证新消息可见
	var visibleMsgs []model.ChatMessage
	db.Where("session_id = ?", sess.ID).Order("id ASC").Find(&visibleMsgs)
	if len(visibleMsgs) != 2 {
		t.Fatalf("期望 2 条新消息，实际 %d", len(visibleMsgs))
	}
	if visibleMsgs[0].Content != "new-q" {
		t.Errorf("第一条消息应为 new-q，实际 %q", visibleMsgs[0].Content)
	}
	if visibleMsgs[1].Content != "new-a" {
		t.Errorf("第二条消息应为 new-a，实际 %q", visibleMsgs[1].Content)
	}

	// 验证会话统计更新
	repo.UpdateSessionStats(sess.ID, time.Now(), len(newMsgs)-int(deletedCount))
	var updatedSess model.ChatSession
	db.First(&updatedSess, sess.ID)
	if updatedSess.MessageCount != 2 {
		t.Errorf("期望 MessageCount=2，实际 %d", updatedSess.MessageCount)
	}
}

func TestNormalizeChatMessageJSONFields_DefaultsEmptyToolCalls(t *testing.T) {
	msgs := []model.ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi", ToolCalls: "   "},
		{Role: "assistant", Content: "tool call", ToolCalls: `[{"id":"call_1"}]`},
	}

	normalizeChatMessageJSONFields(msgs)

	if msgs[0].ToolCalls != "[]" {
		t.Fatalf("expected empty ToolCalls to default to [], got %q", msgs[0].ToolCalls)
	}
	if msgs[1].ToolCalls != "[]" {
		t.Fatalf("expected whitespace ToolCalls to default to [], got %q", msgs[1].ToolCalls)
	}
	if msgs[2].ToolCalls != `[{"id":"call_1"}]` {
		t.Fatalf("expected existing ToolCalls to be preserved, got %q", msgs[2].ToolCalls)
	}
}

func TestCreateMessages_DefaultsEmptyToolCallsForJSONB(t *testing.T) {
	db := setupTestDB(t)
	repo := NewChatRepo(db)
	sess := createTestSession(t, db)

	msgs := []model.ChatMessage{
		{SessionID: sess.ID, UserID: 9999, Role: "user", Content: "hello"},
		{SessionID: sess.ID, UserID: 9999, Role: "assistant", Content: "hi"},
	}
	if err := repo.CreateMessages(msgs); err != nil {
		t.Fatalf("CreateMessages should accept empty ToolCalls by defaulting it to []: %v", err)
	}

	var visibleMsgs []model.ChatMessage
	if err := db.Where("session_id = ?", sess.ID).Order("id ASC").Find(&visibleMsgs).Error; err != nil {
		t.Fatalf("failed to read saved messages: %v", err)
	}
	if len(visibleMsgs) != 2 {
		t.Fatalf("expected 2 saved messages, got %d", len(visibleMsgs))
	}
	for _, msg := range visibleMsgs {
		if msg.ToolCalls != "[]" {
			t.Fatalf("expected saved ToolCalls to be [], got %q", msg.ToolCalls)
		}
	}
}
