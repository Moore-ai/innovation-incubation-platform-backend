package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/internal/model"
)

func (s *AIService) collectFileSummaries(ctx context.Context, policy *model.Policy) []string {
	if policy.Requirements == nil {
		return nil
	}
	var summaries []string
	var mu sync.Mutex
	g, ctx := errgroup.WithContext(ctx)
	for _, basis := range policy.Requirements.LegalBasis {
		g.Go(func() error {
			// 文件名 + 条款始终参与
			line := "- " + basis.Title
			if basis.SpecificClause != "" {
				line += "\n  依据条款：" + basis.SpecificClause
			}
			// 文件解析后的原始文本（RawText）仅在配置开启时用于生成/获取摘要
			if s.useLegalRawForSummary {
				file, err := s.fileRepo.FindByID(basis.FileID)
				if err == nil {
					if file.Summary == "" && file.RawText != "" {
						if err := s.SummarizeFile(ctx, file); err != nil {
							slog.Warn("summarize file failed", "file_id", basis.FileID, "error", err)
						} else {
							file, _ = s.fileRepo.FindByID(basis.FileID)
						}
					}
					if file.Summary != "" {
						line += "\n  文件摘要：" + file.Summary
					}
				}
			}
			mu.Lock()
			summaries = append(summaries, line)
			mu.Unlock()
			return nil
		})
	}
	g.Wait()
	return summaries
}

func (s *AIService) ExtractPolicy(ctx context.Context, policy *model.Policy) error {
	var msg string
	msg += fmt.Sprintf("政策标题：%s\n", policy.Title)
	msg += fmt.Sprintf("政策内容：%s\n", toJSONString(policy.Requirements))

	legalSummaries := s.collectFileSummaries(ctx, policy)
	if len(legalSummaries) > 0 {
		msg += "政策依据文件摘要：\n" + strings.Join(legalSummaries, "\n") + "\n"
	}

	msg += fmt.Sprintf("\n\n严格按照以下 JSON 格式返回（字符串字段必须用双引号括起来，数组字段必须用方括号），不要附带其他内容：\n%s",
		`{"policy_name":"政策名称","policy_summary":"政策概括，200字以内","applicable_industries":["适用行业列表"],"applicable_scales":["适用企业规模，如大型、中型、小型、微型"],"applicable_status":"适用企业状态，如：初创期、成长期","subsidy_type":"补贴类型","subsidies":[{"condition":"补贴条件","amount":"补贴金额","amount_min":数值(纯数字,省略单位万),"amount_max":数值(纯数字,省略单位万)}],"applicable_region":"适用区域","required_documents":["所需材料清单"]}`,
	)

	fields, err := chatAndParse[model.ExtractedPolicy](s, ctx, "extract", s.prompts.extract, msg, "AI提取结果解析失败")
	if err != nil {
		return err
	}
	policy.ExtractedFields = fields
	return nil
}

func cleanLLMOutput(s string) string {
	s = strings.TrimPrefix(s, "\ufeff")
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"```json", "```"} {
		if idx := strings.Index(s, prefix); idx >= 0 {
			s = s[idx+len(prefix):]
			break
		}
	}
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if start := strings.IndexByte(s, '{'); start >= 0 {
		if end := strings.LastIndexByte(s, '}'); end >= start {
			s = s[start : end+1]
		}
	}
	return s
}

func toJSONString(v any) string {
	if v == nil {
		return "{}"
	}
	b, _ := json.Marshal(v)
	if len(b) == 0 || string(b) == "null" {
		return "{}"
	}
	return string(b)
}

func (s *AIService) buildEmbeddingText(p *model.Policy, legalFiles []model.File) string {
	var parts []string
	parts = append(parts, p.Title)
	if p.ExtractedFields != nil {
		if p.ExtractedFields.PolicySummary != "" {
			parts = append(parts, p.ExtractedFields.PolicySummary)
		}
		for _, s := range p.ExtractedFields.Subsidies {
			parts = append(parts, "补贴："+s.Condition+"，"+s.Amount)
		}
	}
	if p.Requirements != nil {
		if p.Requirements.ApplicationCondition != nil {
			parts = append(parts, *p.Requirements.ApplicationCondition)
		}
		if p.Requirements.FulfillmentCriteria != nil {
			parts = append(parts, *p.Requirements.FulfillmentCriteria)
		}
		if p.Requirements.Process != nil {
			parts = append(parts, *p.Requirements.Process)
		}
		// 文件名称 + 具体条款始终参与向量化
		summaries := make(map[uint]string, len(legalFiles))
		if s.useLegalRawForEmbedding {
			for _, f := range legalFiles {
				if f.Summary != "" {
					summaries[f.ID] = f.Summary
				}
			}
		}
		for _, basis := range p.Requirements.LegalBasis {
			text := basis.Title
			if basis.SpecificClause != "" {
				text += "：" + basis.SpecificClause
			}
			if s, ok := summaries[basis.FileID]; ok {
				text += "。" + s
			}
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "。")
}
