package service

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

type AIService struct {
	cm      *aiclient.AnthropicChatModel
	entRepo *repository.EnterpriseRepo
	govRepo *repository.GovernmentRepo
	prompts struct {
		extract string
		match   string
		prefill string
	}
}

func NewAIService(cm *aiclient.AnthropicChatModel, entRepo *repository.EnterpriseRepo, govRepo *repository.GovernmentRepo, cfg *config.Config) *AIService {
	return &AIService{
		cm:      cm,
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
