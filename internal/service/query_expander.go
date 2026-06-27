package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"innovation-incubation-platform-backend/pkg/aiclient"
)

var listMarkerRe = regexp.MustCompile(`^\s*[-*•]?\s*\d*[.)]\s*`)

type QueryExpander struct {
	client *aiclient.Client
	n      int
}

func NewQueryExpander(client *aiclient.Client, n int) *QueryExpander {
	return &QueryExpander{client: client, n: n}
}

func (e *QueryExpander) Expand(ctx context.Context, query string) ([]string, error) {
	systemPrompt := "你是检索查询扩展助手。生成语义等价或互补的多样化查询。使用中文，简短，避免标点。"
	userMsg := fmt.Sprintf("原始查询：%s\n请严格给出%d个不同表述的查询，每行一个，不要多余内容。", query, e.n)

	text, err := e.client.Chat(ctx, systemPrompt, userMsg)
	if err != nil {
		slog.Warn("query expand failed, using original", "error", err)
		return []string{query}, err
	}

	variants := parseVariants(text, e.n)
	result := make([]string, 0, 1+len(variants))
	result = append(result, query)
	result = append(result, variants...)
	return result, nil
}

func parseVariants(text string, max int) []string {
	lines := strings.Split(text, "\n")
	var variants []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = listMarkerRe.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		variants = append(variants, line)
		if len(variants) >= max {
			break
		}
	}
	return variants
}
