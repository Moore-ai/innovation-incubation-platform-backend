package memory

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

// embedder SemanticMemory 所需的 embedding 能力。
type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// semanticRepo SemanticMemory 所需的仓储方法。
type semanticRepo interface {
	CreateSemanticMemory(m *model.SemanticMemory) error
	RetrieveSemanticByVector(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error)
}

type SemanticMemory struct {
	repo          semanticRepo
	aiClient      *aiclient.Client // 用于 HyDE 生成（可为 nil）
	embedClient   embedder         // 用于向量化（可为 nil）
	semanticLimit int
	hydeMaxTokens int
}

func NewSemanticMemory(repo semanticRepo, aiClient *aiclient.Client, embedClient *aiclient.EmbeddingClient, limit int, hydeMaxTokens int) *SemanticMemory {
	var embed embedder
	if embedClient != nil {
		embed = embedClient
	}
	return &SemanticMemory{
		repo:          repo,
		aiClient:      aiClient,
		embedClient:   embed,
		semanticLimit: limit,
		hydeMaxTokens: hydeMaxTokens,
	}
}

const hydePrompt = `根据用户输入，生成一段假设的语义记忆条目，用于向量检索。

条目风格举例：
- 用户偏好：生成报告时默认使用 PDF 格式
- 用户偏好：回复要简洁
- 教训：query_enterprise_info 缺少企业名，应提醒用户提供

直接输出假设条目，不要分析过程。如果没有明确的偏好或教训线索则输出空字符串。`

func (m *SemanticMemory) Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = m.semanticLimit
	}

	// 1. HyDE: 生成假设记忆条目
	hydeDoc := m.generateHydeDoc(ctx, query)

	// 2. 向量检索（用假设条目或原始查询）
	searchText := hydeDoc
	if searchText == "" {
		searchText = query
	}
	if searchText == "" {
		return nil, nil
	}

	var items []*MemoryItem
	if m.embedClient != nil {
		vec, err := m.embedClient.Embed(ctx, searchText)
		if err == nil {
			vecResults, err := m.repo.RetrieveSemanticByVector(opts.UserID, vec, limit)
			if err == nil {
				seen := make(map[uint]bool)
				for _, mem := range vecResults {
					if seen[mem.ID] {
						continue
					}
					seen[mem.ID] = true
					items = append(items, &MemoryItem{Content: mem.Content, Importance: mem.Importance, Source: "semantic"})
				}
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (m *SemanticMemory) generateHydeDoc(ctx context.Context, query string) string {
	if m.aiClient == nil || query == "" {
		return ""
	}
	userMsg := fmt.Sprintf("用户输入：%s", query)
	text, err := m.aiClient.ChatWithMaxTokens(ctx, hydePrompt, userMsg, m.hydeMaxTokens)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(text)
}

