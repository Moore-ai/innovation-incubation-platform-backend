package memory

import (
	"strings"
	"testing"
	"time"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

func init() {
	tokenutil.SetEstimationMode("simple")
}

// --- appendSegment ---

func TestAppendSegment_EmptyItems(t *testing.T) {
	var parts []string
	budget := appendSegment(&parts, 100, "### Header\n", []string{}, func(s string) string { return s })
	if budget != 100 {
		t.Errorf("expected budget 100, got %d", budget)
	}
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(parts))
	}
}

func TestAppendSegment_ZeroBudget(t *testing.T) {
	var parts []string
	budget := appendSegment(&parts, 0, "### Header\n", []string{"a"}, func(s string) string { return s })
	if budget != 0 {
		t.Errorf("expected budget 0, got %d", budget)
	}
	if len(parts) != 0 {
		t.Errorf("expected 0 parts, got %d", len(parts))
	}
}

func TestAppendSegment_NegativeBudget(t *testing.T) {
	var parts []string
	budget := appendSegment(&parts, -5, "### Header\n", []string{"a"}, func(s string) string { return s })
	if budget != -5 {
		t.Errorf("expected budget -5, got %d", budget)
	}
}

func TestAppendSegment_HeaderExceedsBudget(t *testing.T) {
	var parts []string
	header := "### A very long header that exceeds budget\n"
	headerTokens := tokenutil.Estimate(header)
	budget := appendSegment(&parts, headerTokens-1, header, []string{"item"}, func(s string) string { return s })
	if budget != headerTokens-1 {
		t.Errorf("expected budget unchanged (%d), got %d", headerTokens-1, budget)
	}
	if len(parts) != 0 {
		t.Errorf("expected 0 parts when header exceeds budget")
	}
}

func TestAppendSegment_AllItemsFit(t *testing.T) {
	var parts []string
	items := []string{"- a\n", "- b\n"}
	header := "### H\n"
	// simple mode: header "### H\n" → CJK=0, fields=["###","H"] → 2 tokens
	// item "- a\n" → CJK=0, fields=["-","a"] → 2 tokens
	// item "- b\n" → CJK=0, fields=["-","b"] → 2 tokens
	// total = 2 + 2 + 2 = 6
	totalTokens := tokenutil.Estimate(header) + tokenutil.Estimate(items[0]) + tokenutil.Estimate(items[1])
	budget := appendSegment(&parts, totalTokens+10, header, items, func(s string) string { return s })

	if budget <= 0 {
		t.Errorf("expected positive remaining budget, got %d", budget)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if !strings.Contains(parts[0], header) {
		t.Error("part missing header")
	}
	if !strings.Contains(parts[0], items[0]) {
		t.Error("part missing item 0")
	}
	if !strings.Contains(parts[0], items[1]) {
		t.Error("part missing item 1")
	}
}

func TestAppendSegment_PartialItemsDueToBudget(t *testing.T) {
	var parts []string
	items := []string{"- first\n", "- second\n", "- third\n"}
	header := "### H\n"
	// Budget: header + only first 2 items
	budget := tokenutil.Estimate(header) + tokenutil.Estimate(items[0]) + tokenutil.Estimate(items[1])
	remaining := appendSegment(&parts, budget, header, items, func(s string) string { return s })

	if remaining < 0 {
		t.Errorf("remaining budget should not be negative: %d", remaining)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if !strings.Contains(parts[0], items[0]) {
		t.Error("part missing item 0")
	}
	if !strings.Contains(parts[0], items[1]) {
		t.Error("part missing item 1")
	}
	if strings.Contains(parts[0], items[2]) {
		t.Error("item 2 should not fit in budget")
	}
}

func TestAppendSegment_CustomFormatFn(t *testing.T) {
	var parts []string
	items := []int{10, 20}
	header := "### N\n"
	budget := tokenutil.Estimate(header) + tokenutil.Estimate("N10\n") + tokenutil.Estimate("N20\n") + 10
	appendSegment(&parts, budget, header, items, func(n int) string {
		return "N" + strings.Repeat("x", n) + "\n"
	})
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
}

// --- rankWithDecay ---

func TestRankWithDecay_Empty(t *testing.T) {
	result := rankWithDecay(nil, nil, 0.9, 5)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

func TestRankWithDecay_SingleMessage(t *testing.T) {
	now := time.Now()
	msgs := []model.ChatMessage{
		{BaseModel: model.BaseModel{CreatedAt: now}, Content: "hello"},
	}
	distances := []float64{0.2}
	result := rankWithDecay(msgs, distances, 0.9, 5)
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", result[0].Content)
	}
}

func TestRankWithDecay_LowerScoreFirst(t *testing.T) {
	now := time.Now()
	msgs := []model.ChatMessage{
		{BaseModel: model.BaseModel{CreatedAt: now}, Content: "far"},
		{BaseModel: model.BaseModel{CreatedAt: now}, Content: "near"},
	}
	// distance 0.1 → sim 0.9; distance 0.8 → sim 0.2
	distances := []float64{0.8, 0.1}
	result := rankWithDecay(msgs, distances, 1.0, 5) // decay=1 → no time effect
	if result[0].Content != "far" {
		t.Errorf("expected 'far' first (lower score), got %q", result[0].Content)
	}
}

func TestRankWithDecay_OlderPenalized(t *testing.T) {
	now := time.Now()
	msgs := []model.ChatMessage{
		{BaseModel: model.BaseModel{CreatedAt: now}, Content: "recent"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -3, 0)}, Content: "old"}, // 90 days ago
	}
	// Both same similarity, but "old" should be penalized
	// decay=0.5: old→0.5^(90/30)=0.5^3=0.125; recent→0.5^(0/30)=1.0
	distances := []float64{0.2, 0.2}
	result := rankWithDecay(msgs, distances, 0.5, 5)
	if result[0].Content != "old" {
		t.Errorf("expected 'old' first (lower decayed score), got %q", result[0].Content)
	}
}

func TestRankWithDecay_LimitTruncation(t *testing.T) {
	now := time.Now()
	msgs := make([]model.ChatMessage, 10)
	distances := make([]float64, 10)
	for i := range 10 {
		msgs[i] = model.ChatMessage{
			BaseModel: model.BaseModel{CreatedAt: now},
			Content:   "msg",
		}
		distances[i] = float64(i) * 0.1
	}
	result := rankWithDecay(msgs, distances, 1.0, 3)
	if len(result) != 3 {
		t.Errorf("expected 3 (limit), got %d", len(result))
	}
}

func TestRankWithDecay_ZeroDistanceIsPerfect(t *testing.T) {
	now := time.Now()
	msgs := []model.ChatMessage{
		{BaseModel: model.BaseModel{CreatedAt: now}, Content: "perfect"},
	}
	distances := []float64{0.0}
	result := rankWithDecay(msgs, distances, 1.0, 5)
	if len(result) != 1 {
		t.Fatal("expected 1 result")
	}
	// sim = 1.0 - 0.0 = 1.0, should rank highest
	if result[0].Content != "perfect" {
		t.Errorf("expected 'perfect', got %q", result[0].Content)
	}
}

func TestRankWithDecay_NoDecayWhenZero(t *testing.T) {
	now := time.Now()
	msgs := []model.ChatMessage{
		{BaseModel: model.BaseModel{CreatedAt: now}, Content: "a"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -12, 0)}, Content: "b"},
	}
	distances := []float64{0.3, 0.5}
	result := rankWithDecay(msgs, distances, 0.0, 5)
	// decay=0 → timeWeight=1 for both, pure similarity ranking
	// sim: a=0.7, b=0.5
	if result[0].Content != "b" {
		t.Errorf("expected 'b' first (lower similarity score, no decay), got %q", result[0].Content)
	}
}

// --- buildOrdered ---

func TestBuildOrdered_Empty(t *testing.T) {
	result := buildOrdered(nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestBuildOrdered_SingleLine(t *testing.T) {
	result := buildOrdered([]string{"hello\n"})
	if result != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result)
	}
}

func TestBuildOrdered_ReversesLines(t *testing.T) {
	result := buildOrdered([]string{"third\n", "second\n", "first\n"})
	expected := "first\nsecond\nthird\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
