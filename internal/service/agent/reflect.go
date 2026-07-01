package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/pkg/aiclient"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

type ReflectChecker struct {
	embedClient *aiclient.EmbeddingClient
	threshold   float64
	toolDescs   map[string][]float32 // 工具名 → 预缓存的静态描述 embedding
}

func NewReflectChecker(embedClient *aiclient.EmbeddingClient, registry *agenttools.ToolRegistry, cfg config.ReflectConfig) *ReflectChecker {
	rc := &ReflectChecker{
		embedClient: embedClient,
		threshold:   cfg.SimilarityThreshold,
		toolDescs:   make(map[string][]float32),
	}
	// 预计算每个工具的静态行为描述 embedding
	for _, t := range registry.All() {
		if embedClient != nil {
			desc := fmt.Sprintf("工具 %s：%s。预期返回：%s", t.Name(), t.Description(), string(t.OutputSchema()))
			ectx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			vec, err := embedClient.Embed(ectx, desc)
			cancel()
			if err == nil {
				rc.toolDescs[t.Name()] = vec
			}
		}
	}
	return rc
}

// Check 返回 (是否触发反思, 触发原因)
func (r *ReflectChecker) Check(ctx context.Context, toolName string, result json.RawMessage, execErr error) (bool, string) {
	// 第一层：硬规则
	if execErr != nil {
		return true, fmt.Sprintf("工具执行出错: %v", execErr)
	}
	if len(result) == 0 || string(result) == "null" || string(result) == `""` {
		return true, "工具返回为空"
	}

	s := string(result)
	for _, kw := range []string{"权限不足", "无权限", "forbidden", "unauthorized", "内部错误", "服务不可用"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(kw)) {
			return true, fmt.Sprintf("工具返回包含异常关键词: %s", kw)
		}
	}

	// 第二层：Output Schema 校验（若 OutputSchema 不为空）
	// 注：此处做基础 JSON 格式校验，完整的 JSON Schema 校验可后续扩展
	if !json.Valid(result) {
		return true, "工具返回不是合法的 JSON"
	}

	// 第三层：Embedding 语义相似度
	if r.embedClient != nil {
		descVec, ok := r.toolDescs[toolName]
		if ok {
			obsVec, err := r.embedClient.Embed(ctx, s)
			if err == nil {
				sim := cosineSimilarity(descVec, obsVec)
				if sim < r.threshold {
					return true, fmt.Sprintf("语义相似度过低(%.4f < %.2f)", sim, r.threshold)
				}
			}
		}
	}

	return false, ""
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
