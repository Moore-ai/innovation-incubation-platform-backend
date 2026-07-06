package memory

import (
	"context"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

// embedder SemanticMemory 所需的 embedding 能力。
type embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// semanticRepo SemanticMemory 所需的仓储方法。
type semanticRepo interface {
	CreateSemanticMemory(m *model.SemanticMemory) error
	RetrieveSemanticByCategory(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error)
	RetrieveSemanticByVector(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error)
}

type SemanticMemory struct {
	repo          semanticRepo
	embedClient   embedder
	semanticLimit int
}

func NewSemanticMemory(repo *repository.ChatRepo, embedClient *aiclient.EmbeddingClient, limit int) *SemanticMemory {
	var embed embedder
	if embedClient != nil {
		embed = embedClient
	}
	return &SemanticMemory{repo: repo, embedClient: embed, semanticLimit: limit}
}

func (m *SemanticMemory) Add(ctx context.Context, item *MemoryItem) error {
	mem := &model.SemanticMemory{
		Content:    item.Content,
		Importance: item.Importance,
		Category:   "lesson",
	}
	if m.embedClient != nil {
		vec, _ := m.embedClient.Embed(ctx, item.Content)
		mem.Embedding = vec
	}
	return m.repo.CreateSemanticMemory(mem)
}

func (m *SemanticMemory) Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = m.semanticLimit
	}

	// 先从精准匹配中提取关键词
	var keyword string
	words := strings.Fields(query)
	if len(words) > 0 {
		keyword = words[0]
	}

	// 精准匹配: category in ('rule','preference')
	exact, err := m.repo.RetrieveSemanticByCategory(opts.UserID, []string{"rule", "preference"}, keyword, limit)
	if err != nil {
		exact = nil
	}

	// 向量检索补充
	var vecResults []model.SemanticMemory
	if m.embedClient != nil {
		vec, err := m.embedClient.Embed(ctx, query)
		if err == nil {
			vecResults, _ = m.repo.RetrieveSemanticByVector(opts.UserID, vec, limit)
		}
	}

	// 合并去重
	seen := make(map[uint]bool)
	var items []*MemoryItem
	for _, m := range exact {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		items = append(items, &MemoryItem{Content: m.Content, Importance: m.Importance, Source: "semantic"})
	}
	for _, m := range vecResults {
		if seen[m.ID] {
			continue
		}
		seen[m.ID] = true
		items = append(items, &MemoryItem{Content: m.Content, Importance: m.Importance, Source: "semantic"})
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (m *SemanticMemory) Clear(ctx context.Context) error { return nil }
