package aiclient

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type AnthropicChatModel struct {
	client *Client
}

func NewAnthropicChatModel(c *Client) *AnthropicChatModel {
	return &AnthropicChatModel{client: c}
}

// Generate implements BaseChatModel[*schema.Message].
// Known limitation: opts ...model.Option (callbacks, model params) are not forwarded to the Anthropic API.
func (m *AnthropicChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	var systemPrompt string
	var parts []string

	for _, msg := range input {
		switch msg.Role {
		case schema.System:
			systemPrompt = msg.Content
		case schema.User:
			parts = append(parts, msg.Content)
		case schema.Assistant:
			// skipped: single-turn payload, no history to inject
		}
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no user message")
	}

	userMsg := strings.Join(parts, "\n\n")
	text, err := m.client.Chat(ctx, systemPrompt, userMsg)
	if err != nil {
		slog.Error("eino generate failed", "error", err)
		return nil, err
	}

	return schema.AssistantMessage(text, nil), nil
}

func (m *AnthropicChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("stream not supported")
}

var _ model.BaseChatModel = (*AnthropicChatModel)(nil)
