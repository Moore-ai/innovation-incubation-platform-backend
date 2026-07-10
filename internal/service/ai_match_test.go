package service

import (
	"strings"
	"testing"

	"innovation-incubation-platform-backend/internal/model"
)

func TestFallbackMatchForPolicyUsesLocalKeywords(t *testing.T) {
	ent := &model.Enterprise{
		Name:        "合肥软件科技有限公司",
		Industry:    "信息传输、软件和信息技术服务业",
		Description: "从事工业互联网平台和人工智能应用开发",
	}
	policy := model.Policy{Title: "培育省级重点工业互联网平台"}

	match := fallbackMatchForPolicy(ent, policy)

	if match.Level == "unknown" {
		t.Fatalf("expected local fallback to produce a concrete match level, got unknown: %#v", match)
	}
	if match.Level != "high" && match.Level != "partial" {
		t.Fatalf("expected high or partial match, got %q", match.Level)
	}
	if !strings.Contains(match.Reason, "AI暂不可用") {
		t.Fatalf("expected fallback reason to mention AI unavailable, got %q", match.Reason)
	}
}

func TestFallbackMatchForPolicyCanReturnNone(t *testing.T) {
	ent := &model.Enterprise{
		Industry:    "信息传输、软件和信息技术服务业",
		Description: "从事软件开发",
	}
	policy := model.Policy{Title: "鼓励类外资项目进口自用设备免税确认"}

	match := fallbackMatchForPolicy(ent, policy)

	if match.Level != "none" {
		t.Fatalf("expected no local keyword overlap to be none, got %q: %s", match.Level, match.Reason)
	}
}

func TestNormalizeMatchLevelAcceptsChineseLabels(t *testing.T) {
	cases := map[string]string{
		"高匹配":  "high",
		"部分匹配": "partial",
		"不匹配":  "none",
		"未知":   "unknown",
	}
	for input, want := range cases {
		if got := normalizeMatchLevel(input); got != want {
			t.Fatalf("normalizeMatchLevel(%q) = %q, want %q", input, got, want)
		}
	}
}
