package builtin

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

var _ agenttools.Tool = (*RecordSemanticMemory)(nil)

type RecordSemanticMemory struct {
	chatRepo    *repository.ChatRepo
	embedClient *aiclient.EmbeddingClient
}

func NewRecordSemanticMemory(chatRepo *repository.ChatRepo, embedClient *aiclient.EmbeddingClient) *RecordSemanticMemory {
	return &RecordSemanticMemory{chatRepo: chatRepo, embedClient: embedClient}
}

func (t *RecordSemanticMemory) Name() string { return "record_semantic_memory" }

func (t *RecordSemanticMemory) Description() string {
	return "记录本轮对话中用户表达的偏好和工具调用的教训。lessons 记录工具失败教训，preferences 记录用户偏好。无可留空数组。"
}

func (t *RecordSemanticMemory) AllowedRoles() []string {
	return []string{"enterprise", "carrier", "government"}
}

func (t *RecordSemanticMemory) Timeout() time.Duration { return 10 * time.Second }

func (t *RecordSemanticMemory) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"lessons":{"type":"array","items":{"type":"string"},"description":"工具调用失败的教训，每条不超过80字"},
			"preferences":{"type":"array","items":{"type":"string"},"description":"用户表达的偏好，每条不超过80字"}
		}
	}`)
}

func (t *RecordSemanticMemory) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"success":{"type":"boolean"},"saved":{"type":"integer"}}}`)
}

type recordInput struct {
	Lessons     []string `json:"lessons"`
	Preferences []string `json:"preferences"`
}

func (t *RecordSemanticMemory) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input recordInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	userID := agenttools.UserIDFromCtx(ctx)
	saved := 0

	for _, lesson := range input.Lessons {
		if lesson = trimStr(lesson); lesson == "" {
			continue
		}
		if err := t.save(ctx, userID, lesson, string(agentmemory.CategoryLesson)); err == nil {
			saved++
		}
	}
	for _, pref := range input.Preferences {
		if pref = trimStr(pref); pref == "" {
			continue
		}
		if err := t.save(ctx, userID, pref, string(agentmemory.CategoryPreference)); err == nil {
			saved++
		}
	}

	b, _ := json.Marshal(map[string]any{"success": true, "saved": saved})
	return json.RawMessage(b), nil
}

func (t *RecordSemanticMemory) save(ctx context.Context, userID uint, content, category string) error {
	mem := &model.SemanticMemory{
		Content:    content,
		Category:   category,
		Importance: 0.5,
		UserID:     &userID,
	}
	if t.embedClient != nil {
		vec, err := t.embedClient.Embed(ctx, content)
		if err != nil {
			slog.Warn("semantic memory embedding failed", "error", err)
		} else {
			mem.Embedding = vec
		}
	}
	return t.chatRepo.CreateSemanticMemory(mem)
}

func trimStr(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
