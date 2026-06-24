package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// chatAndParse sends a Chat request and unmarshals the response into the target type T.
// Returns typed pointer on success, or an errcode.ErrAIService error on failure.
func chatAndParse[T any](s *AIService, ctx context.Context, opName, systemPrompt, userMsg, parseErrMsg string) (*T, error) {
	text, err := s.client.Chat(ctx, systemPrompt, userMsg)
	if err != nil {
		slog.Warn("AI chat failed", "op", opName, "error", err)
		return nil, errcode.ErrAIService.WithMsg("AI服务暂不可用")
	}
	var result T
	if err := json.Unmarshal([]byte(cleanLLMOutput(text)), &result); err != nil {
		slog.Error("AI parse failed", "op", opName, "error", err)
		return nil, errcode.ErrAIService.WithMsg(parseErrMsg)
	}
	return &result, nil
}

type AIService struct {
	client  *aiclient.Client
	entRepo *repository.EnterpriseRepo
	govRepo *repository.GovernmentRepo
	prompts struct {
		extract string
		match   string
		prefill string
	}
}

func NewAIService(client *aiclient.Client, entRepo *repository.EnterpriseRepo, govRepo *repository.GovernmentRepo, cfg *config.Config) *AIService {
	return &AIService{
		client:  client,
		entRepo: entRepo,
		govRepo: govRepo,
		prompts: struct {
			extract string
			match   string
			prefill string
		}{
			extract: cfg.AI.Prompts.Extract,
			match:   cfg.AI.Prompts.Match,
			prefill: cfg.AI.Prompts.Prefill,
		},
	}
}
