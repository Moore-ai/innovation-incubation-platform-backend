package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/internal/model"
)

type extractedFields struct {
	PolicyName           string   `json:"policy_name"`           // 政策名称
	PolicySummary        string   `json:"policy_summary"`        // 政策概括
	ApplicableIndustries []string `json:"applicable_industries"` // 适用行业
	ApplicableScales     []string `json:"applicable_scales"`     // 适用企业规模
	ApplicableStatus     string   `json:"applicable_status"`     // 适用企业状态（如：初创期、成长期）
	SubsidyType          string   `json:"subsidy_type"`          // 补贴类型（如：资金补贴、税收优惠）
	SubsidyAmount        string   `json:"subsidy_amount"`        // 补贴金额
	SubsidyCondition     string   `json:"subsidy_condition"`     // 补贴条件
	ApplicableRegion     string   `json:"applicable_region"`     // 适用区域；也可能是 JSON 数组
	RequiredDocuments    []string `json:"required_documents"`    // 所需材料清单
}

func (s *AIService) ensureFileSummaries(ctx context.Context, policy *model.Policy) error {
	if policy.Requirements == nil {
		return nil
	}
	g, ctx := errgroup.WithContext(ctx)
	for _, basis := range policy.Requirements.LegalBasis {
		g.Go(func() error {
			file, err := s.fileRepo.FindByID(basis.FileID)
			if err != nil {
				return nil // 文件不存在，跳过
			}
			if file.Summary != "" || file.RawText == "" {
				return nil // 已有摘要或无法生成，跳过
			}
			return s.SummarizeFile(ctx, file)
		})
	}
	return g.Wait()
}

func (s *AIService) ExtractPolicy(ctx context.Context, policy *model.Policy) error {
	// 1. 确保所有法律依据文件有摘要（并发生成）
	if err := s.ensureFileSummaries(ctx, policy); err != nil {
		slog.Warn("ensure file summaries failed, continuing without summaries", "error", err)
	}

	// 2. 构建 userMsg
	var msg string
	msg += fmt.Sprintf("政策标题：%s\n", policy.Title)
	msg += fmt.Sprintf("政策内容：%s\n", toJSONString(policy.Requirements))

	// 3. 收集法律依据文件摘要
	var legalSummaries []string
	if policy.Requirements != nil {
		for _, basis := range policy.Requirements.LegalBasis {
			file, err := s.fileRepo.FindByID(basis.FileID)
			if err == nil && file.Summary != "" {
				legalSummaries = append(legalSummaries,
					fmt.Sprintf("- %s：%s", basis.Title, file.Summary))
			}
		}
	}
	if len(legalSummaries) > 0 {
		msg += "政策依据文件摘要：\n" + strings.Join(legalSummaries, "\n") + "\n"
	}

	msg += fmt.Sprintf("\n\n严格按照以下 JSON 格式返回（字符串字段必须用双引号括起来，数组字段必须用方括号），不要附带其他内容：\n%s",
		`{"policy_name":"政策名称","policy_summary":"政策概括，200字以内","applicable_industries":["适用行业列表"],"applicable_scales":["适用企业规模，如大型、中型、小型、微型"],"applicable_status":"适用企业状态，如：初创期、成长期","subsidy_type":"补贴类型","subsidy_amount":"补贴金额","subsidy_condition":"补贴的具体条件","applicable_region":"适用区域","required_documents":["所需材料清单"]}`,
	)

	fields, err := chatAndParse[extractedFields](s, ctx, "extract", s.prompts.extract, msg, "AI提取结果解析失败")
	if err != nil {
		return err
	}
	b, _ := json.Marshal(fields)
	var extracted model.JSONMap
	json.Unmarshal(b, &extracted)
	policy.ExtractedFields = extracted
	return nil
}

// cleanLLMOutput extracts JSON content from markdown code block wrapping.
func cleanLLMOutput(s string) string {
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
	return strings.TrimSpace(s)
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
