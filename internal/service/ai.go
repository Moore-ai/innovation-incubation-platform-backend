package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/errcode"
)

type PromptSet struct {
	extract        string
	match          string
	prefill        string
	summarize      string
	search         string
	searchAnalysis string
}

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
	prompts      PromptSet
	maxFileChars int
}

func NewAIService(client *aiclient.Client, entRepo *repository.EnterpriseRepo, govRepo *repository.GovernmentRepo, fileRepo *repository.FileRepo, cfg *config.Config) *AIService {
	return &AIService{
		client:       client,
		entRepo:      entRepo,
		govRepo:      govRepo,
		fileRepo:     fileRepo,
		fileMatchCfg: cfg.FileMatch,
		prompts: PromptSet{
			extract:        cfg.AI.Prompts.Extract,
			match:          cfg.AI.Prompts.Match,
			prefill:        cfg.AI.Prompts.Prefill,
			summarize:      cfg.AI.Prompts.Summarize,
			search:         cfg.AI.Prompts.Search,
			searchAnalysis: cfg.AI.Prompts.SearchAnalysis,
		},
		maxFileChars: cfg.AI.MaxFileChars,
	}
}

type summaryResult struct {
	Text string `json:"text"`
}

type analysisResult struct {
	Text      string `json:"text"`
	RankedIDs []uint `json:"ranked_ids"`
	Found     bool   `json:"found"`
	Effect    string `json:"effect"`
}

type analyzeText struct {
	Text   string `json:"text"`
	Found  bool   `json:"found"`
	Effect string `json:"effect"`
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

// buildPolicyBriefs formats policies into a human-readable brief list for AI prompts.
func buildPolicyBriefs(policies []model.Policy) []string {
	briefs := make([]string, 0, len(policies))
	for _, p := range policies {
		title, amount, deadline, summary := p.Title, "", "", ""
		if ef := p.ExtractedFields; ef != nil {
			var amts []string
			for _, s := range ef.Subsidies {
				if s.Amount != "" {
					amts = append(amts, s.Amount)
				}
			}
			amount = strings.Join(amts, ",")
			summary = ef.PolicySummary
		}
		if p.EndDate != "" {
			deadline = p.EndDate
		}
		brief := fmt.Sprintf("[%d]「%s」补贴%s，截止%s，摘要：%s", p.ID, title, amount, deadline, summary)
		briefs = append(briefs, brief)
	}
	return briefs
}

// AnalyzeAndRankResults uses AI to analyze and rank search results.
// Returns re-ordered policies, analysis text, ranked IDs, and effect evaluation.
func (s *AIService) AnalyzeAndRankResults(ctx context.Context, query string, ent *model.Enterprise, policies []model.Policy) ([]model.Policy, *analysisResult, error) {
	if len(policies) == 0 {
		userMsg := fmt.Sprintf("企业信息：行业=%s、规模=%s、地址=%s\n"+
			"用户搜索：%s\n\n"+
			"本次检索未找到匹配的政策。请分析可能的原因并给出建议。\n"+
			"严格按照以下 JSON 格式返回，不要附带其他内容：(注意，ranked_ids必须是一个空数组，即[])\n"+
			`{"text":"你的分析内容，200字以内","ranked_ids":[],"found":false,"effect":"low"}`,
			ent.Industry, ent.Scale, ent.Address, query)
		r, err := chatAndParse[analysisResult](s, ctx, "search_analysis", s.prompts.searchAnalysis, userMsg, "AI分析失败")
		if err != nil {
			return nil, nil, err
		}
		return policies, r, nil
	}

	briefs := buildPolicyBriefs(policies)
	userMsg := fmt.Sprintf(
		"企业信息：行业=%s、规模=%s、地址=%s\n用户搜索：%s\n\n以下是数据库中关键词匹配到的相关政策：\n%s\n\n"+
			"请分析这些政策是否真正满足用户需求（尤其是金额、时间等精确条件）。\n"+
			"如果满足，给出个性化的推荐理由和注意事项（包括补贴金额是否匹配、截止时间是否充裕等）；\n"+
			"如果不满足，说明具体原因（如金额超出预算、截止时间太近等）。\n"+
			"最后还要评估本次回答是否能满足用户的要求，按照high|partial|low打分\n"+
			"严格按照以下 JSON 格式返回，不要附带其他内容：\n"+
			`{"text":"你的分析内容，200字以内","ranked_ids":[最匹配的ID,按推荐度降序],"found":true,"effect":"high、partial或者low，分别代表高、一般、低，用于评估本次检索的效果"}`,
		ent.Industry, ent.Scale, ent.Address, query, strings.Join(briefs, "\n"))
	r, err := chatAndParse[analysisResult](s, ctx, "search_analysis", s.prompts.searchAnalysis, userMsg, "AI分析失败")
	if err != nil {
		return nil, nil, err
	}

	// 按 ranked_ids 重排
	rankedIDs := r.RankedIDs
	if len(rankedIDs) > 0 {
		ranked := make([]model.Policy, 0, len(policies))
		seen := make(map[uint]bool, len(rankedIDs))
		for _, id := range rankedIDs {
			for _, p := range policies {
				if p.ID == id && !seen[id] {
					ranked = append(ranked, p)
					seen[id] = true
					break
				}
			}
		}
		for _, p := range policies {
			if !seen[p.ID] {
				ranked = append(ranked, p)
			}
		}
		return ranked, r, nil
	}
	return policies, r, nil
}

// AnalyzeResults uses AI to analyze search results without re-ranking.
// Returns analysis text and effect evaluation only (no ranked_ids).
func (s *AIService) AnalyzeResults(ctx context.Context, query string, ent *model.Enterprise, policies []model.Policy) (*analyzeText, error) {
	if len(policies) == 0 {
		userMsg := fmt.Sprintf("企业信息：行业=%s、规模=%s、地址=%s\n"+
			"用户搜索：%s\n\n"+
			"本次检索未找到匹配的政策。请分析可能的原因并给出建议。\n"+
			"严格按照以下 JSON 格式返回，不要附带其他内容：\n"+
			`{"text":"你的分析内容，200字以内","found":false,"effect":"low"}`,
			ent.Industry, ent.Scale, ent.Address, query)
		r, err := chatAndParse[analyzeText](s, ctx, "search_analysis", s.prompts.searchAnalysis, userMsg, "AI分析失败")
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	briefs := buildPolicyBriefs(policies)
	userMsg := fmt.Sprintf(
		"企业信息：行业=%s、规模=%s、地址=%s\n用户搜索：%s\n\n以下是数据库中关键词匹配到的相关政策：\n%s\n\n"+
			"请分析这些政策是否真正满足用户需求（尤其是金额、时间等精确条件）。\n"+
			"如果满足，给出个性化的推荐理由和注意事项（包括补贴金额是否匹配、截止时间是否充裕等）；\n"+
			"如果不满足，说明具体原因（如金额超出预算、截止时间太近等）。\n"+
			"最后还要评估本次回答是否能满足用户的要求，按照high|partial|low打分\n"+
			"严格按照以下 JSON 格式返回，不要附带其他内容：\n"+
			`{"text":"你的分析内容，200字以内","found":true,"effect":"high、partial或者low，分别代表高、一般、低，用于评估本次检索的效果"}`,
		ent.Industry, ent.Scale, ent.Address, query, strings.Join(briefs, "\n"))
	r, err := chatAndParse[analyzeText](s, ctx, "search_analysis", s.prompts.searchAnalysis, userMsg, "AI分析失败")
	if err != nil {
		return nil, err
	}
	return r, nil
}
