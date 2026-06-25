package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
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
	client       *aiclient.Client
	entRepo      *repository.EnterpriseRepo
	govRepo      *repository.GovernmentRepo
	fileRepo     *repository.FileRepo
	fileMatchCfg config.FileMatchConfig
	prompts      struct {
		extract   string
		match     string
		prefill   string
		summarize string
		search    string
	}
	maxFileChars int
}

func NewAIService(client *aiclient.Client, entRepo *repository.EnterpriseRepo, govRepo *repository.GovernmentRepo, fileRepo *repository.FileRepo, cfg *config.Config) *AIService {
	return &AIService{
		client:   client,
		entRepo:  entRepo,
		govRepo:  govRepo,
		fileRepo: fileRepo,
		fileMatchCfg: cfg.FileMatch,
		prompts: struct {
			extract   string
			match     string
			prefill   string
			summarize string
			search    string
		}{
			extract:   cfg.AI.Prompts.Extract,
			match:     cfg.AI.Prompts.Match,
			prefill:   cfg.AI.Prompts.Prefill,
			summarize: cfg.AI.Prompts.Summarize,
			search:    cfg.AI.Prompts.Search,
		},
		maxFileChars: cfg.AI.MaxFileChars,
	}
}

type summaryResult struct {
	Text string `json:"text"`
}

func (s *AIService) SummarizeFile(ctx context.Context, file *model.File) error {
	text := file.RawText
	if text == "" {
		return nil
	}
	if s.maxFileChars > 0 && len(text) > s.maxFileChars {
		text = text[:s.maxFileChars]
	}
	result, err := chatAndParse[summaryResult](s, ctx, "summarize", s.prompts.summarize,
		fmt.Sprintf("请概括以下政策文件的核心内容：\n\n%s", text),
		"AI摘要生成失败")
	if err != nil {
		return err
	}
	file.Summary = result.Text
	if err := s.fileRepo.UpdateSummary(file.ID, result.Text); err != nil {
		slog.Error("file update summary failed", "file_id", file.ID, "error", err)
		return err
	}
	return nil
}
