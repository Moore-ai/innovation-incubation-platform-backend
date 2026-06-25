package filematch

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
)

// stopWords 文件名中无意义的后缀词
var stopWords = []string{"复印件", "原件", "扫描件", "照片", "图片", "副本", "电子版", "扫描"}

var tokenRe = regexp.MustCompile(`[\p{Han}a-zA-Z0-9]+`)

// MatchResult 文件匹配结果
type MatchResult struct {
	FileID  uint
	Score   float64
	Warning string
}

// Search 对 query 与 files 列表执行三维评分匹配，返回按分数降序排列的结果
func Search(query string, formats []string, files []model.File, cfg config.FileMatchConfig) []MatchResult {
	var results []MatchResult
	queryTokens := tokenize(query)

	for _, f := range files {
		score := calculateScore(f.Filename, query, queryTokens, cfg)

		if score < cfg.Threshold {
			continue
		}

		var warning string
		if len(formats) > 0 {
			fext := ext(f.Filename)
			match := false
			for _, f2 := range formats {
				if strings.EqualFold(fext, f2) {
					match = true
					break
				}
			}
			if !match {
				warning = fmt.Sprintf("文件「%s」扩展名不匹配，要求 %s 格式", f.Filename, strings.Join(formats, "/"))
			}
		}

		results = append(results, MatchResult{FileID: f.ID, Score: score, Warning: warning})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results
}

// Match 对 materialName 与 files 列表执行三维评分匹配，返回最佳匹配结果
func Match(materialName string, formats []string, files []model.File, cfg config.FileMatchConfig) *MatchResult {
	results := Search(materialName, formats, files, cfg)
	if len(results) == 0 {
		return nil
	}
	return &results[0]
}

// tokenize 分词，去除扩展名和停用词，保留中文/英文/数字片段
func tokenize(s string) []string {
	s = strings.ToLower(s)
	for _, sw := range stopWords {
		s = strings.ReplaceAll(s, sw, "")
	}
	return tokenRe.FindAllString(s, -1)
}

// ext 返回文件扩展名（不含点）
func ext(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		return filename[idx+1:]
	}
	return ""
}

// nameOnly 返回不含扩展名的文件名
func nameOnly(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		return filename[:idx]
	}
	return filename
}

// calculateScore 三维评分：Jaro-Winkler + 关键词 + 前缀
func calculateScore(filename, query string, queryTokens []string, cfg config.FileMatchConfig) float64 {
	name := nameOnly(filename)

	sjw := jaroWinkler(name, query) * cfg.WeightJaro
	skw := keywordScore(tokenize(name), queryTokens) * cfg.WeightKeyword
	sp := prefixScore(name, query) * cfg.WeightPrefix

	return sjw + skw + sp
}

// keywordScore 关键词匹配分数，短词（≤2字符）权重 ×1.5
func keywordScore(fileTokens, queryTokens []string) float64 {
	if len(queryTokens) == 0 || len(fileTokens) == 0 {
		return 0
	}
	matched := 0.0
	totalWeight := 0.0

	for _, qt := range queryTokens {
		weight := 1.0
		if len([]rune(qt)) <= 2 {
			weight = 1.5
		}
		totalWeight += weight

		for _, ft := range fileTokens {
			if ft == qt {
				matched += weight
				break
			}
			if strings.Contains(ft, qt) || strings.Contains(qt, ft) {
				matched += weight * 0.7
				break
			}
		}
	}

	if totalWeight == 0 {
		return 0
	}
	return matched / totalWeight
}

// prefixScore 前缀匹配：逐字符比较开头部分
func prefixScore(filename, query string) float64 {
	fr := []rune(filename)
	qr := []rune(query)
	maxLen := min(len(fr), len(qr))
	if maxLen == 0 {
		return 0
	}
	match := 0
	for i := 0; i < maxLen; i++ {
		if fr[i] == qr[i] {
			match++
		} else {
			break
		}
	}
	return float64(match) / float64(len(qr))
}
