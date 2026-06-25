package filematch

import (
	"fmt"
	"strings"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
)

// stopWords 文件名中无意义的后缀词
var stopWords = []string{"复印件", "原件", "扫描件", "照片", "图片", "副本", "电子版", "扫描"}

func extractKeywords(s string) []string {
	s = strings.ToLower(s)
	for _, sw := range stopWords {
		s = strings.ReplaceAll(s, sw, "")
	}
	// 按空格和标点分词
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '（' || r == '）' || r == '(' || r == ')' ||
			r == '-' || r == '_' || r == '.' || r == '，' || r == '。' || r == '、'
	})
	var result []string
	for _, w := range words {
		w = strings.TrimSpace(w)
		if w != "" {
			result = append(result, w)
		}
	}
	return result
}

func keywordScore(materialKW, fileKW []string) float64 {
	if len(materialKW) == 0 {
		return 0
	}
	matched := 0
	for _, mw := range materialKW {
		for _, fw := range fileKW {
			if strings.Contains(mw, fw) || strings.Contains(fw, mw) {
				matched++
				break
			}
		}
	}
	return float64(matched) / float64(len(materialKW))
}

func prefixScore(filename, materialName string) float64 {
	fl := len(filename) / 3
	ml := len(materialName) / 3
	if fl == 0 || ml == 0 {
		return 0
	}
	return jaro(filename[:fl], materialName[:ml])
}

// MatchResult 文件匹配结果
type MatchResult struct {
	FileID  uint
	Score   float64
	Warning string
}

// Match 对 materialName 与 files 列表执行三维评分匹配，返回最佳匹配结果
func Match(materialName string, formats []string, files []model.File, cfg config.FileMatchConfig) *MatchResult {
	var best *MatchResult
	materialKW := extractKeywords(materialName)

	for _, f := range files {
		filename := strings.TrimSuffix(f.Filename, ext(f.Filename))

		// 三维评分
		sjw := jaroWinkler(filename, materialName)
		skw := keywordScore(materialKW, extractKeywords(filename))
		sp := prefixScore(filename, materialName)

		total := sjw*cfg.WeightJaro + skw*cfg.WeightKeyword + sp*cfg.WeightPrefix

		if total < cfg.Threshold {
			continue
		}

		// 检查扩展名
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

		if best == nil || total > best.Score {
			best = &MatchResult{FileID: f.ID, Score: total, Warning: warning}
		}
	}
	return best
}

func ext(filename string) string {
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		return filename[idx+1:]
	}
	return ""
}
