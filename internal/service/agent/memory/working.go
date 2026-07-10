package memory

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

// workingRepo WorkingMemory 所需的仓储方法。
type workingRepo interface {
	LoadMessagesPage(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error)
}

type WorkingMemory struct {
	repo     workingRepo
	pageSize int
}

func NewWorkingMemory(repo workingRepo, pageSize int) *WorkingMemory {
	return &WorkingMemory{repo: repo, pageSize: pageSize}
}

func (m *WorkingMemory) Add(ctx context.Context, item *MemoryItem) error {
	return nil
}

// Deprecated: use BuildWorkingContext.
func (m *WorkingMemory) Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error) {
	return nil, fmt.Errorf("WorkingMemory.Retrieve is deprecated, use BuildWorkingContext instead")
}

func (m *WorkingMemory) Clear(ctx context.Context) error { return nil }

// BuildWorkingContext 按 Token 预算分页加载历史消息，返回时间升序的上下文文本。
// excludeID > 0 时跳过 ID >= excludeID 的消息（用于编辑重发场景）。
func (m *WorkingMemory) BuildWorkingContext(sessionID uint, budget int, excludeID uint) (string, error) {
	var cursor uint
	var lines []string
	used := 0

	for budget-used > 0 {
		msgs, nextCursor, hasMore, err := m.repo.LoadMessagesPage(sessionID, cursor, m.pageSize)
		if err != nil {
			return "", err
		}
		for _, msg := range msgs {
			if excludeID > 0 && msg.ID >= excludeID {
				continue
			}
			line := msg.Role + ": " + msg.Content + "\n"
			tokens := tokenutil.Estimate(line)
			if used+tokens > budget {
				continue
			}
			used += tokens
			lines = append(lines, line)
		}
		if !hasMore {
			break
		}
		cursor = nextCursor
	}
	return buildOrdered(lines), nil
}

// buildOrdered 将逆序收集的行反转为时间升序
func buildOrdered(lines []string) string {
	var sb strings.Builder
	for i := len(lines) - 1; i >= 0; i-- {
		sb.WriteString(lines[i])
	}
	return sb.String()
}
