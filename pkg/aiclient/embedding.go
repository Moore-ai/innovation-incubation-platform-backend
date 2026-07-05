package aiclient

import (
	"context"
	"fmt"
	"log/slog"

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
	var lastErr error
	for attempt := 0; attempt <= defaultRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("Embedding 重试", "attempt", attempt, "max", defaultRetries)
		}
		resp, err := c.inner.CreateEmbeddings(ctx, openai.EmbeddingRequest{
			Input: []string{text},
			Model: openai.EmbeddingModel(c.model),
		})
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				break
			}
			continue
		}
		if len(resp.Data) == 0 {
			lastErr = fmt.Errorf("embedding API returned empty data")
			continue
		}
		return resp.Data[0].Embedding, nil
	}
	slog.Error("embedding failed", "error", lastErr)
	return nil, lastErr
}
