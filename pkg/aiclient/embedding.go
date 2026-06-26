package aiclient

import (
	"context"
	openai "github.com/sashabaranov/go-openai"

	"innovation-incubation-platform-backend/config"
)

type EmbeddingClient struct {
	inner *openai.Client
	model string
}

func NewEmbeddingClient(cfg config.EmbeddingConfig) *EmbeddingClient {
	ocfg := openai.DefaultConfig(cfg.APIKey)
	ocfg.BaseURL = cfg.BaseURL
	return &EmbeddingClient{
		inner: openai.NewClientWithConfig(ocfg),
		model: cfg.Model,
	}
}

func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.inner.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}
