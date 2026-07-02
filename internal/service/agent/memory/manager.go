package memory

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/config"
)

type MemoryManager struct {
	working  *WorkingMemory
	semantic *SemanticMemory
	cfg      config.AgentConfig
}

func NewMemoryManager(working *WorkingMemory, semantic *SemanticMemory, cfg config.AgentConfig) *MemoryManager {
	return &MemoryManager{working: working, semantic: semantic, cfg: cfg}
}

// LoadContext 加载上下文，返回注入 System Prompt 的文本块
func (m *MemoryManager) LoadContext(ctx context.Context, sessionID uint, userID uint, query string) (string, error) {
	var parts []string

	// 工作记忆
	budget := int(float64(m.cfg.ContextWindow) * m.cfg.HistoryBudgetRatio)
	wctx, err := m.working.BuildWorkingContext(sessionID, budget)
	if err != nil {
		return "", fmt.Errorf("working memory: %w", err)
	}
	if wctx != "" {
		parts = append(parts, "### 对话历史\n"+wctx)
	}

	// 语义记忆
	items, err := m.semantic.Retrieve(ctx, query, RetrievalOpts{UserID: userID, Limit: m.cfg.Memory.SemanticLimit})
	if err != nil {
		// 降级：跳过语义记忆
		items = nil
	}
	if len(items) > 0 {
		var sb strings.Builder
		sb.WriteString("### 相关规则与偏好\n")
		for _, item := range items {
			sb.WriteString("- ")
			sb.WriteString(item.Content)
			sb.WriteString("\n")
		}
		parts = append(parts, sb.String())
	}

	return strings.Join(parts, "\n\n"), nil
}

// AddSemantic 写入语义记忆（外部触发）
func (m *MemoryManager) AddSemantic(ctx context.Context, content string, importance float64, category string) error {
	return m.semantic.Add(ctx, &MemoryItem{
		Content:    content,
		Importance: importance,
	})
}
