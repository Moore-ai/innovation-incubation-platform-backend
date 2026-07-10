package service

import (
	"context"
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
)

// PolicyMatchResult represents the LLM-based matching outcome for a policy.
type PolicyMatchResult struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

type policyMatchListResult struct {
	Matches []policyMatchListItem `json:"matches"`
}

type policyMatchListItem struct {
	PolicyID uint   `json:"policy_id"`
	Level    string `json:"level"`
	Reason   string `json:"reason"`
}

// MatchPolicy performs LLM-based policy matching for an enterprise against a specific policy.
func (s *AIService) MatchPolicy(ctx context.Context, userID uint, policyID uint) (*PolicyMatchResult, error) {
	ent, err := s.entRepo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	policy, err := s.govRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}

	result, err := s.matchOnePolicy(ctx, ent, *policy)
	if err != nil {
		return fallbackMatchForPolicy(ent, *policy), nil
	}
	return result, nil
}

func (s *AIService) MatchPolicyListForEnterprise(ctx context.Context, ent *model.Enterprise, policies []model.Policy) map[uint]PolicyMatchResult {
	results := make(map[uint]PolicyMatchResult, len(policies))
	if ent == nil || len(policies) == 0 {
		return results
	}
	briefs := make([]string, 0, len(policies))
	for _, policy := range policies {
		briefs = append(briefs, buildPolicyMatchBrief(policy))
	}
	userMsg := fmt.Sprintf("%s\n\n以下是待匹配政策：\n%s\n\n请只根据企业所属领域和企业简介判断每个政策的匹配度；如果企业简介为空，只根据所属领域判断。严格返回 JSON，不要附带其他内容：\n%s",
		enterprisePolicyMatchProfile(ent),
		strings.Join(briefs, "\n---\n"),
		`{"matches":[{"policy_id":1,"level":"high|partial|none|unknown","reason":"简短说明匹配依据"}]}`,
	)
	result, err := ChatAndParse[policyMatchListResult](s, ctx, "match_list", s.prompts.match, userMsg, "AI匹配失败")
	if err != nil {
		for _, policy := range policies {
			results[policy.ID] = *fallbackMatchForPolicy(ent, policy)
		}
		return results
	}
	for _, item := range result.Matches {
		results[item.PolicyID] = PolicyMatchResult{Level: normalizeMatchLevel(item.Level), Reason: item.Reason}
	}
	for _, policy := range policies {
		if _, ok := results[policy.ID]; !ok {
			results[policy.ID] = *fallbackMatchForPolicy(ent, policy)
		}
	}
	return results
}

func (s *AIService) matchOnePolicy(ctx context.Context, ent *model.Enterprise, policy model.Policy) (*PolicyMatchResult, error) {
	userMsg := fmt.Sprintf("%s\n%s\n\n请只根据企业所属领域和企业简介判断政策匹配度；如果企业简介为空，只根据所属领域判断。严格返回 JSON，不要附带其他内容：\n%s",
		enterprisePolicyMatchProfile(ent),
		buildPolicyMatchBrief(policy),
		`{"level":"high|partial|none|unknown","reason":"说明所属领域、企业简介与政策适用对象/支持方向的匹配依据"}`,
	)
	result, err := ChatAndParse[PolicyMatchResult](s, ctx, "match", s.prompts.match, userMsg, "AI匹配失败")
	if err != nil {
		return nil, err
	}
	result.Level = normalizeMatchLevel(result.Level)
	return result, nil
}

func buildPolicyMatchBrief(policy model.Policy) string {
	extractedFields := any(policy.Requirements)
	if policy.ExtractedFields != nil {
		extractedFields = policy.ExtractedFields
	}
	return fmt.Sprintf("policy_id=%d\n政策标题=%s\n政策条件=%s\n提取字段=%s",
		policy.ID, policy.Title, toJSONString(policy.Requirements), toJSONString(extractedFields))
}

func enterprisePolicyMatchProfile(ent *model.Enterprise) string {
	industry := strings.TrimSpace(ent.Industry)
	description := strings.TrimSpace(ent.Description)
	if description == "" {
		return fmt.Sprintf("企业画像：所属领域=%s；企业简介未填写，仅根据所属领域进行政策匹配。", industry)
	}
	return fmt.Sprintf("企业画像：所属领域=%s；企业简介=%s。", industry, description)
}

func normalizeMatchLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "high", "高", "高匹配":
		return "high"
	case "partial", "medium", "middle", "中", "中匹配", "部分匹配":
		return "partial"
	case "none", "no", "low", "低", "不匹配", "无匹配":
		return "none"
	case "unknown", "未知", "无法判断":
		return "unknown"
	default:
		return "unknown"
	}
}

func fallbackMatchForPolicy(ent *model.Enterprise, policy model.Policy) *PolicyMatchResult {
	if ent == nil {
		return fallbackMatch()
	}
	keywords := enterpriseMatchKeywords(ent)
	if len(keywords) == 0 {
		return &PolicyMatchResult{
			Level:  "unknown",
			Reason: "AI暂不可用，企业画像缺少行业和简介，暂无法判断匹配度",
		}
	}

	policyText := strings.ToLower(strings.Join([]string{
		policy.Title,
		toJSONString(policy.Requirements),
		toJSONString(policy.ExtractedFields),
	}, "\n"))

	if level, reason, ok := matchExtractedIndustries(policy.ExtractedFields, keywords); ok {
		return &PolicyMatchResult{Level: level, Reason: reason}
	}

	hits := matchingKeywords(keywords, policyText)
	switch {
	case len(hits) >= 2:
		return &PolicyMatchResult{
			Level:  "high",
			Reason: "AI暂不可用，已按企业行业/简介与政策关键词进行本地匹配：" + strings.Join(hits[:2], "、"),
		}
	case len(hits) == 1:
		return &PolicyMatchResult{
			Level:  "partial",
			Reason: "AI暂不可用，已按企业画像进行本地匹配，命中关键词：" + hits[0],
		}
	default:
		return &PolicyMatchResult{
			Level:  "none",
			Reason: "AI暂不可用，本地规则未发现企业行业/简介与政策方向的明显重合",
		}
	}
}

func fallbackMatch() *PolicyMatchResult {
	return &PolicyMatchResult{
		Level:  "unknown",
		Reason: "AI暂不可用",
	}
}

func enterpriseMatchKeywords(ent *model.Enterprise) []string {
	text := strings.ToLower(strings.Join([]string{ent.Industry, ent.Description, ent.Name}, " "))
	keywords := make([]string, 0, 12)
	add := func(values ...string) {
		for _, value := range values {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "" {
				continue
			}
			exists := false
			for _, item := range keywords {
				if item == value {
					exists = true
					break
				}
			}
			if !exists {
				keywords = append(keywords, value)
			}
		}
	}

	add(strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == ',' || r == '，' || r == '、' || r == ';' || r == '；' || r == '/' || r == '|' || r == '\n'
	})...)
	if strings.Contains(text, "信息传输") || strings.Contains(text, "软件") || strings.Contains(text, "信息技术") || strings.Contains(text, "电子信息") {
		add("软件", "信息技术", "数字", "互联网", "人工智能", "大数据", "5g", "电子信息", "信息服务", "智能")
	}
	if strings.Contains(text, "制造") || strings.Contains(text, "工业") {
		add("制造", "工业", "智能制造", "先进制造", "工厂", "工业互联网")
	}
	if strings.Contains(text, "科研") || strings.Contains(text, "科学研究") || strings.Contains(text, "技术服务") || strings.Contains(text, "研发") {
		add("研发", "科研", "科技", "实验室", "成果转化", "技术服务")
	}
	if strings.Contains(text, "绿色") || strings.Contains(text, "环保") || strings.Contains(text, "节能") {
		add("绿色", "环保", "节能", "低碳")
	}
	if strings.Contains(text, "生物") || strings.Contains(text, "医药") {
		add("生物", "医药", "医疗")
	}
	if strings.Contains(text, "新能源") || strings.Contains(text, "新材料") {
		add("新能源", "新材料")
	}
	if strings.Contains(text, "金融") || strings.Contains(text, "租赁") || strings.Contains(text, "融资") {
		add("金融", "租赁", "融资", "贴息")
	}
	if strings.Contains(text, "外资") || strings.Contains(text, "进口") {
		add("外资", "进口", "设备")
	}
	return keywords
}

func matchExtractedIndustries(fields *model.ExtractedPolicy, keywords []string) (string, string, bool) {
	if fields == nil || len(fields.ApplicableIndustries) == 0 {
		return "", "", false
	}
	industryText := strings.ToLower(strings.Join(fields.ApplicableIndustries, " "))
	if strings.Contains(industryText, "不限") || strings.Contains(industryText, "所有") || strings.Contains(industryText, "各类") {
		return "partial", "AI暂不可用，政策适用行业较宽，本地规则判断为部分匹配", true
	}
	hits := matchingKeywords(keywords, industryText)
	if len(hits) > 0 {
		return "high", "AI暂不可用，政策适用行业与企业画像匹配：" + strings.Join(hits[:min(2, len(hits))], "、"), true
	}
	return "", "", false
}

func matchingKeywords(keywords []string, text string) []string {
	hits := make([]string, 0, 4)
	for _, keyword := range keywords {
		if len([]rune(keyword)) < 2 {
			continue
		}
		if strings.Contains(text, keyword) {
			hits = append(hits, keyword)
		}
	}
	return hits
}
