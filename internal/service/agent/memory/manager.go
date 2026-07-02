package memory

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/pkg/tokenutil"
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
// 按预算驱动：优先加载语义记忆，剩余预算用于工作记忆
func (m *MemoryManager) LoadContext(ctx context.Context, sessionID, userID uint, query string, budget int) (string, error) {
	var parts []string

	// 1. 语义记忆（优先）
	if budget > 0 {
		items, err := m.semantic.Retrieve(ctx, query, RetrievalOpts{UserID: userID, Limit: m.cfg.Memory.SemanticLimit})
		if err != nil {
			// 降级：跳过语义记忆
			items = nil
		}
		if len(items) > 0 {
			var sb strings.Builder
			headerLine := "### 相关规则与偏好\n"
			headerTokens := tokenutil.ApproxTokenLen(headerLine)
			if headerTokens <= budget {
				budget -= headerTokens
				sb.WriteString(headerLine)
				for _, item := range items {
					line := "- " + item.Content + "\n"
					tokens := tokenutil.ApproxTokenLen(line)
					if tokens > budget {
						break
					}
					budget -= tokens
					sb.WriteString(line)
				}
				if sb.Len() > 0 {
					parts = append(parts, sb.String())
				}
			}
		}
	}

	// 2. 工作记忆（剩余预算）
	if budget > 0 {
		wctx, err := m.working.BuildWorkingContext(sessionID, budget)
		if err != nil {
			if len(parts) > 0 {
				return strings.Join(parts, "\n\n"), nil
			}
			return "", fmt.Errorf("working memory: %w", err)
		}
		if wctx != "" {
			parts = append(parts, "### 对话历史\n"+wctx)
		}
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
