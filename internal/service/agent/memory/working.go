package memory

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/internal/repository"
)

type WorkingMemory struct {
	repo     *repository.ChatRepo
	capacity int
}

func NewWorkingMemory(repo *repository.ChatRepo, capacity int) *WorkingMemory {
	return &WorkingMemory{repo: repo, capacity: capacity}
}

func (m *WorkingMemory) Add(ctx context.Context, item *MemoryItem) error {
	return nil // 工作记忆不独立写入——消息通过 chat_messages 表持久化
}

func (m *WorkingMemory) Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error) {
	msgs, err := m.repo.LoadRecentMessages(opts.UserID, m.capacity)
	if err != nil {
		return nil, err
	}
	items := make([]*MemoryItem, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Content == "" {
			continue
		}
		items = append(items, &MemoryItem{
			Content: fmt.Sprintf("[%s]: %s", msg.Role, msg.Content),
			Source:  "working",
		})
	}
	return items, nil
}

func (m *WorkingMemory) Clear(ctx context.Context) error { return nil }

// BuildWorkingContext 直接返回拼接好的工作记忆文本（跳过 MemoryItem 中间层）
func (m *WorkingMemory) BuildWorkingContext(sessionID uint) (string, error) {
	msgs, err := m.repo.LoadRecentMessages(sessionID, m.capacity)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, msg := range msgs {
		sb.WriteString(msg.Role)
		sb.WriteString(": ")
		sb.WriteString(msg.Content)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
