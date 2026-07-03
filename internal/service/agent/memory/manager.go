package memory

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

type MemoryManager struct {
	working     *WorkingMemory
	semantic    *SemanticMemory
	repo        *repository.ChatRepo
	embedClient *aiclient.EmbeddingClient
	cfg         config.AgentConfig
}

func NewMemoryManager(working *WorkingMemory, semantic *SemanticMemory, repo *repository.ChatRepo, embedClient *aiclient.EmbeddingClient, cfg config.AgentConfig) *MemoryManager {
	return &MemoryManager{working: working, semantic: semantic, repo: repo, embedClient: embedClient, cfg: cfg}
}

// LoadContext 加载上下文：语义记忆 → 情景记忆（向量检索）→ 工作记忆。
// 用户查询的 embedding 在入口计算一次，语义和情景共享。
func (m *MemoryManager) LoadContext(ctx context.Context, sessionID, userID uint, query string, budget int) (string, error) {
	var queryVec []float32
	if m.embedClient != nil && budget > 0 {
		vec, err := m.embedClient.Embed(ctx, query)
		if err == nil {
			queryVec = vec
		}
	}

	var parts []string

	// 1. 语义记忆（优先）
	if budget > 0 {
		items, err := m.semantic.Retrieve(ctx, query, RetrievalOpts{UserID: userID, Limit: m.cfg.Memory.SemanticLimit})
		if err != nil {
			items = nil
		}
		if len(items) > 0 {
			var sb strings.Builder
			headerLine := "### 相关规则与偏好\n"
			headerTokens := tokenutil.Estimate(headerLine)
			if headerTokens <= budget {
				budget -= headerTokens
				sb.WriteString(headerLine)
				for _, item := range items {
					line := "- " + item.Content + "\n"
					tokens := tokenutil.Estimate(line)
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

	// 2. 情景记忆（向量语义相似度检索）
	if budget > 0 && m.cfg.Memory.EpisodicLimit > 0 && len(queryVec) > 0 {
		msgs, err := m.repo.SearchMessagesByVector(userID, queryVec, m.cfg.Memory.EpisodicLimit)
		if err != nil {
			msgs = nil
		}
		if len(msgs) > 0 {
			var sb strings.Builder
			headerLine := "### 相关历史对话\n"
			headerTokens := tokenutil.Estimate(headerLine)
			if headerTokens <= budget {
				budget -= headerTokens
				sb.WriteString(headerLine)
				for _, msg := range msgs {
					line := msg.Role + ": " + msg.Content + "\n"
					tokens := tokenutil.Estimate(line)
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

	// 3. 工作记忆（剩余预算）
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
