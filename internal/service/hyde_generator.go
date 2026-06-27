package service

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/pkg/aiclient"
)

// HyDEGenerator calls an LLM to generate a hypothetical document from a query,
// which is then used for vector search retrieval.
type HyDEGenerator struct {
	client *aiclient.Client
}

func NewHyDEGenerator(client *aiclient.Client) *HyDEGenerator {
	return &HyDEGenerator{client: client}
}

func (g *HyDEGenerator) Generate(ctx context.Context, query string) (string, error) {
	systemPrompt := "根据用户问题，先写一段可能的答案性段落，用于向量检索的查询文档（不要分析过程）。"
	userMsg := fmt.Sprintf("问题：%s\n请直接写一段中等长度、客观、包含关键术语的段落。", query)
	text, err := g.client.ChatWithMaxTokens(ctx, systemPrompt, userMsg, 512)
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("HyDE returned empty")
	}
	return text, nil
}
