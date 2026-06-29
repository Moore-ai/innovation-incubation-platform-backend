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

const defaultRetries = 2

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
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	}
	return retry(ctx, "Chat", func() (string, error) {
		resp, err := c.inner.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("AI返回为空")
		}
		return resp.Choices[0].Message.Content, nil
	})
}

func (c *Client) ChatWithMaxTokens(ctx context.Context, system, user string, maxTokens int) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		MaxTokens: maxTokens,
	}
	return retry(ctx, "ChatWithMaxTokens", func() (string, error) {
		resp, err := c.inner.CreateChatCompletion(ctx, req)
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("AI返回为空")
		}
		return resp.Choices[0].Message.Content, nil
	})
}

func retry(ctx context.Context, op string, fn func() (string, error)) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= defaultRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("AI 调用重试", "op", op, "attempt", attempt, "max", defaultRetries)
		}
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("AI服务超时")
	}
	slog.Error("AI 调用失败", "op", op, "error", lastErr)
	return "", fmt.Errorf("AI服务暂不可用")
}
