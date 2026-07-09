package memory

import (
	"context"
	"testing"

	"innovation-incubation-platform-backend/internal/model"
)

// --- mocks ---

type mockSemanticRepo struct {
	createFn func(m *model.SemanticMemory) error
	vectorFn func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error)
}

func (m *mockSemanticRepo) CreateSemanticMemory(mem *model.SemanticMemory) error {
	if m.createFn != nil {
		return m.createFn(mem)
	}
	return nil
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

func TestNewSemanticMemory_NilEmbeddingClientStaysNil(t *testing.T) {
	sm := NewSemanticMemory(nil, nil, nil, 10, 256)
	if sm.embedClient != nil {
		t.Fatal("nil embedding client should not be stored as a non-nil interface")
	}
	if sm.aiClient != nil {
		t.Fatal("nil ai client should not be stored as non-nil")
	}
	if sm.hydeMaxTokens != 256 {
		t.Fatalf("expected hydeMaxTokens=256, got %d", sm.hydeMaxTokens)
	}
}

func TestSemanticRetrieve_VectorOnly(t *testing.T) {
	repo := &mockSemanticRepo{
		vectorFn: func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 1, Content: "lesson B", Importance: 0.5},
			}, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1, 0.2}},
		aiClient:      nil,
		semanticLimit: 10,
		hydeMaxTokens: 256,
	}

	items, err := sm.Retrieve(context.Background(), "test query", RetrievalOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 vector item, got %d", len(items))
	}
}

func TestSemanticRetrieve_DedupById(t *testing.T) {
	repo := &mockSemanticRepo{
		vectorFn: func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
			return []model.SemanticMemory{
				{ID: 1, Content: "same item", Importance: 0.8},
				{ID: 1, Content: "same item via vector", Importance: 0.5},
			}, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1}},
		aiClient:      nil,
		semanticLimit: 10,
		hydeMaxTokens: 256,
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
		vectorFn: func(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
			items := make([]model.SemanticMemory, 10)
			for i := range 10 {
				items[i] = model.SemanticMemory{ID: uint(i + 1), Content: "item"}
			}
			return items, nil
		},
	}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1}},
		aiClient:      nil,
		semanticLimit: 10,
		hydeMaxTokens: 256,
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
	sm := &SemanticMemory{repo: &mockSemanticRepo{}, embedClient: nil, aiClient: nil, semanticLimit: 10, hydeMaxTokens: 256}

	items, err := sm.Retrieve(context.Background(), "q", RetrievalOpts{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0, got %d", len(items))
	}
}

func TestSemanticRetrieve_EmptyQuery(t *testing.T) {
	repo := &mockSemanticRepo{}
	sm := &SemanticMemory{
		repo:          repo,
		embedClient:   &mockEmbedder{vec: []float32{0.1}},
		aiClient:      nil,
		semanticLimit: 5,
		hydeMaxTokens: 256,
	}

	items, err := sm.Retrieve(context.Background(), "", RetrievalOpts{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 for empty query, got %d", len(items))
	}
}

func TestSemanticRetrieve_HyDEFallback(t *testing.T) {
	// 当 aiClient 和 embedClient 均为 nil 时，Retrieve 应返回空（不 panic）
	sm := &SemanticMemory{
		repo:          &mockSemanticRepo{},
		embedClient:   nil,
		aiClient:      nil,
		hydeMaxTokens: 256,
	}
	items, err := sm.Retrieve(context.Background(), "test", RetrievalOpts{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0, got %d", len(items))
	}
}




