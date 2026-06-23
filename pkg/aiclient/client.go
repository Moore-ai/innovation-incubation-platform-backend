package aiclient

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

type Client struct {
	inner *openai.Client
	model string
}

func New(baseURL, apiKey, model string, timeout int) *Client {
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimRight(baseURL, "/") + "/v1"
	}
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	if timeout > 0 {
		cfg.HTTPClient = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}
	return &Client{
		inner: openai.NewClientWithConfig(cfg),
		model: model,
	}
}

func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	resp, err := c.inner.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	})
	if err != nil {
		slog.Error("ai chat completion failed", "error", err)
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("AI服务超时")
		}
		return "", fmt.Errorf("AI服务暂不可用")
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI返回为空")
	}
	return resp.Choices[0].Message.Content, nil
}
