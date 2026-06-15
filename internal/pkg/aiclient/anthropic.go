package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"innovation-incubation-platform-backend/config"
)

type Client struct {
	cfg  config.AnthropicConfig
	http *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
}

type chatResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func New(cfg config.AnthropicConfig) *Client {
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
	}
}

func (c *Client) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	reqBody := chatRequest{
		Model:     c.cfg.Model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []Message{
			{Role: "user", Content: userMessage},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		slog.Error("ai marshal failed", "error", err)
		return "", fmt.Errorf("ai服务暂不可用")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai服务暂不可用")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Error("ai request failed", "error", err)
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("ai服务超时")
		}
		return "", fmt.Errorf("ai服务暂不可用")
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		slog.Error("ai non-200", "status", resp.StatusCode, "body", string(respBytes))
		return "", fmt.Errorf("ai服务暂不可用")
	}

	var cr chatResponse
	if err := json.Unmarshal(respBytes, &cr); err != nil {
		return "", fmt.Errorf("ai服务暂不可用")
	}

	if len(cr.Content) == 0 {
		return "", fmt.Errorf("ai返回为空")
	}

	return cr.Content[0].Text, nil
}
