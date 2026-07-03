package memory

import (
	"context"
	"strings"
	"testing"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

func init() {
	tokenutil.SetEstimationMode("simple")
}

// --- mocks for MemoryManager ---

type mockWorkingProvider struct {
	ctx string
	err error
}

func (m *mockWorkingProvider) BuildWorkingContext(sessionID uint, budget int) (string, error) {
	return m.ctx, m.err
}

type mockSemanticRetriever struct {
	items []*MemoryItem
	err   error
}

func (m *mockSemanticRetriever) Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error) {
	return m.items, m.err
}

func (m *mockSemanticRetriever) Add(ctx context.Context, item *MemoryItem) error {
	return nil
}

type mockEpisodicRepo struct {
	msgs      []model.ChatMessage
	distances []float64
	err       error
}

func (m *mockEpisodicRepo) SearchMessagesByVectorWithDistance(userID uint, embedding []float32, limit int) ([]model.ChatMessage, []float64, error) {
	return m.msgs, m.distances, m.err
}

// --- tests ---

func newTestCfg() config.AgentConfig {
	return config.AgentConfig{
		Memory: config.AgentMemoryConfig{
			SemanticLimit:      5,
			EpisodicLimit:      3,
			EpisodicDecayFactor: 0,
		},
	}
}

func TestLoadContext_SemanticOnly(t *testing.T) {
	mgr := &MemoryManager{
		working:  &mockWorkingProvider{ctx: ""},
		semantic: &mockSemanticRetriever{
			items: []*MemoryItem{
				{Content: "重要规则: 优先使用 search_policy", Importance: 0.9},
			},
		},
		repo:        &mockEpisodicRepo{},
		embedClient: nil,
		cfg:         newTestCfg(),
	}

	result, err := mgr.LoadContext(context.Background(), 1, 100, "测试查询", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "相关规则与偏好") {
		t.Errorf("expected semantic section, got: %s", result)
	}
	if !strings.Contains(result, "search_policy") {
		t.Errorf("expected semantic item content, got: %s", result)
	}
}

func TestLoadContext_SemanticAndWorking(t *testing.T) {
	mgr := &MemoryManager{
		working: &mockWorkingProvider{
			ctx: "user: 历史消息内容\n",
		},
		semantic: &mockSemanticRetriever{
			items: []*MemoryItem{
				{Content: "偏好规则", Importance: 0.8},
			},
		},
		repo:        &mockEpisodicRepo{},
		embedClient: nil,
		cfg:         newTestCfg(),
	}

	result, err := mgr.LoadContext(context.Background(), 1, 100, "query", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "相关规则与偏好") {
		t.Error("missing semantic section")
	}
	if !strings.Contains(result, "对话历史") {
		t.Error("missing working context section")
	}
}

func TestLoadContext_ZeroBudget(t *testing.T) {
	mgr := &MemoryManager{
		working:  &mockWorkingProvider{ctx: "should not appear"},
		semantic: &mockSemanticRetriever{items: nil},
		repo:     &mockEpisodicRepo{},
		embedClient: &mockEmbedder{vec: []float32{0.1}},
		cfg:      newTestCfg(),
	}

	result, err := mgr.LoadContext(context.Background(), 1, 100, "q", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty with zero budget, got: %s", result)
	}
}

func TestLoadContext_SemanticPriority(t *testing.T) {
	// 语义记忆应该在工作记忆之前加载
	mgr := &MemoryManager{
		working: &mockWorkingProvider{
			ctx: "工作记忆\n",
		},
		semantic: &mockSemanticRetriever{
			items: []*MemoryItem{
				{Content: "语义规则", Importance: 0.9},
			},
		},
		repo:        &mockEpisodicRepo{},
		embedClient: nil,
		cfg:         newTestCfg(),
	}

	result, err := mgr.LoadContext(context.Background(), 1, 100, "q", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	semanticIdx := strings.Index(result, "相关规则与偏好")
	workingIdx := strings.Index(result, "对话历史")
	if semanticIdx < 0 || workingIdx < 0 {
		t.Fatalf("missing sections: %s", result)
	}
	if semanticIdx > workingIdx {
		t.Error("semantic section should appear before working context")
	}
}

func TestLoadContext_TightBudget(t *testing.T) {
	// 预算只够语义记忆的 header，不够任何条目
	headerTokens := tokenutil.Estimate("### 相关规则与偏好\n")
	mgr := &MemoryManager{
		working: &mockWorkingProvider{ctx: ""},
		semantic: &mockSemanticRetriever{
			items: []*MemoryItem{
				{Content: "一条很长的语义记忆条目", Importance: 0.9},
			},
		},
		repo:        &mockEpisodicRepo{},
		embedClient: nil,
		cfg:         newTestCfg(),
	}

	result, err := mgr.LoadContext(context.Background(), 1, 100, "q", headerTokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// header 刚好用完预算，条目放不下 → 工作记忆也放不下
	if strings.Contains(result, "语义记忆条目") {
		t.Error("item should not fit in tight budget")
	}
}
