package memory

import (
	"context"
	"testing"

	"innovation-incubation-platform-backend/internal/model"
)

// --- mocks ---

type mockSemanticRepo struct {
	createFn   func(m *model.SemanticMemory) error
	categoryFn func(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error)
	vectorFn   func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error)
}

func (m *mockSemanticRepo) CreateSemanticMemory(mem *model.SemanticMemory) error {
	if m.createFn != nil {
		return m.createFn(mem)
	}
	return nil
}

func (m *mockSemanticRepo) RetrieveSemanticByCategory(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
	if m.categoryFn != nil {
		return m.categoryFn(userID, categories, keyword, limit)
	}
	return nil, nil
}

func (m *mockSemanticRepo) RetrieveSemanticByVector(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
	if m.vectorFn != nil {
		return m.vectorFn(userID, embedding, limit)
	}
	return nil, nil
}

type mockEmbedder struct {
	vec []float32
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return m.vec, nil
}

// --- tests ---

func TestSemanticRetrieve_CategoryAndVectorMerged(t *testing.T) {
	repo := &mockSemanticRepo{
		categoryFn: func(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 1, Content: "rule A", Importance: 0.8},
			}, nil
		},
		vectorFn: func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 2, Content: "lesson B", Importance: 0.5},
			}, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1, 0.2}},
		semanticLimit: 10,
	}

	items, err := sm.Retrieve(context.Background(), "test query", RetrievalOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (category + vector), got %d", len(items))
	}
}

func TestSemanticRetrieve_DedupById(t *testing.T) {
	repo := &mockSemanticRepo{
		categoryFn: func(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 1, Content: "same item", Importance: 0.8},
			}, nil
		},
		vectorFn: func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 1, Content: "same item via vector", Importance: 0.5},
			}, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1}},
		semanticLimit: 10,
	}

	items, err := sm.Retrieve(context.Background(), "query", RetrievalOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 (deduped), got %d", len(items))
	}
}

func TestSemanticRetrieve_RespectsLimit(t *testing.T) {
	repo := &mockSemanticRepo{
		categoryFn: func(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
			items := make([]model.SemanticMemory, 10)
			for i := range 10 {
				items[i] = model.SemanticMemory{ID: uint(i + 1), Content: "item"}
			}
			return items, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   nil, // 无 embed，只用 category
		semanticLimit: 10,
	}

	items, err := sm.Retrieve(context.Background(), "query", RetrievalOpts{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 (limit), got %d", len(items))
	}
}

func TestSemanticRetrieve_NoEmbedClient(t *testing.T) {
	repo := &mockSemanticRepo{
		categoryFn: func(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 1, Content: "only category", Importance: 0.9},
			}, nil
		},
	}
	sm := &SemanticMemory{repo: repo, embedClient: nil, semanticLimit: 10}

	items, err := sm.Retrieve(context.Background(), "q", RetrievalOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1, got %d", len(items))
	}
}

func TestSemanticRetrieve_EmptyQuery(t *testing.T) {
	repo := &mockSemanticRepo{
		categoryFn: func(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
			return nil, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1}},
		semanticLimit: 5,
	}

	items, err := sm.Retrieve(context.Background(), "", RetrievalOpts{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 for empty query, got %d", len(items))
	}
}
