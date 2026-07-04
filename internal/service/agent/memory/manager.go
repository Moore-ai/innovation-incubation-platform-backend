package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

// episodicRepo 情景记忆所需的仓储方法。
type episodicRepo interface {
	SearchMessagesByVectorWithDistance(userID uint, embedding []float32, limit int) ([]model.ChatMessage, []float64, error)
}

// workingContextProvider 工作记忆的上下文构建能力。
type workingContextProvider interface {
	BuildWorkingContext(sessionID uint, budget int, excludeID uint) (string, error)
}

// semanticRetriever 语义记忆的检索与写入能力。
type semanticRetriever interface {
	Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error)
	Add(ctx context.Context, item *MemoryItem) error
}

type MemoryManager struct {
	working     workingContextProvider
	semantic    semanticRetriever
	repo        episodicRepo
	embedClient embedder
	cfg         config.AgentConfig
}

func NewMemoryManager(working *WorkingMemory, semantic *SemanticMemory, repo *repository.ChatRepo, embedClient *aiclient.EmbeddingClient, cfg config.AgentConfig) *MemoryManager {
	return &MemoryManager{working: working, semantic: semantic, repo: repo, embedClient: embedClient, cfg: cfg}
}

// appendSegment 将 items 按 Token 预算追加到 parts，每条由 formatFn 转换为字符串。
// 返回剩余 budget。
func appendSegment[T any](parts *[]string, budget int, header string, items []T, formatFn func(T) string) int {
	if len(items) == 0 || budget <= 0 {
		return budget
	}
	headerTokens := tokenutil.Estimate(header)
	if headerTokens > budget {
		return budget
	}
	budget -= headerTokens
	var sb strings.Builder
	sb.WriteString(header)
	for _, item := range items {
		line := formatFn(item)
		tokens := tokenutil.Estimate(line)
		if tokens > budget {
			break
		}
		budget -= tokens
		sb.WriteString(line)
	}
	if sb.Len() > 0 {
		*parts = append(*parts, sb.String())
	}
	return budget
}

// LoadContext 加载上下文：语义记忆 → 情景记忆（向量检索）→ 工作记忆。
// excludeID > 0 时从情景/工作记忆中排除 ID >= excludeID 的消息（编辑重发场景）。
func (m *MemoryManager) LoadContext(ctx context.Context, sessionID, userID uint, query string, budget int, excludeID uint) (string, error) {
	var queryVec []float32
	if m.embedClient != nil && budget > 0 {
		vec, err := m.embedClient.Embed(ctx, query)
		if err == nil {
			queryVec = vec
		}
	}

	var parts []string

	// 1. 语义记忆（优先）
	if budget > 0 {
		items, err := m.semantic.Retrieve(ctx, query, RetrievalOpts{UserID: userID, Limit: m.cfg.Memory.SemanticLimit})
		if err != nil {
			items = nil
		}
		budget = appendSegment(&parts, budget, "### 相关规则与偏好\n", items, func(item *MemoryItem) string {
			return "- " + item.Content + "\n"
		})
	}

	// 2. 情景记忆（向量语义相似度 + 时间衰减复合评分）
	if budget > 0 && m.cfg.Memory.EpisodicLimit > 0 && len(queryVec) > 0 {
		fetchLimit := m.cfg.Memory.EpisodicLimit
		if m.cfg.Memory.EpisodicDecayFactor > 0 {
			fetchLimit = m.cfg.Memory.EpisodicLimit * 3
		}
		msgs, distances, err := m.repo.SearchMessagesByVectorWithDistance(userID, queryVec, fetchLimit)
		if err != nil {
			msgs = nil
		}
		// 编辑重发时排除即将被替换的消息（消息和距离数组同步过滤）
		if excludeID > 0 {
			filtered := make([]model.ChatMessage, 0, len(msgs))
			filteredDist := make([]float64, 0, len(distances))
			for i, msg := range msgs {
				if msg.ID < excludeID {
					filtered = append(filtered, msg)
					filteredDist = append(filteredDist, distances[i])
				}
			}
			msgs = filtered
			distances = filteredDist
		}
		if len(msgs) > 0 && m.cfg.Memory.EpisodicDecayFactor > 0 {
			msgs = rankWithDecay(msgs, distances, m.cfg.Memory.EpisodicDecayFactor, m.cfg.Memory.EpisodicLimit)
		}
		budget = appendSegment(&parts, budget, "### 相关历史对话\n", msgs, func(msg model.ChatMessage) string {
			return msg.Role + ": " + msg.Content + "\n"
		})
	}

	// 3. 工作记忆（剩余预算）
	if budget > 0 {
		wctx, err := m.working.BuildWorkingContext(sessionID, budget, excludeID)
		if err != nil {
			if len(parts) > 0 {
				return strings.Join(parts, "\n\n"), nil
			}
			return "", fmt.Errorf("working memory: %w", err)
		}
		if wctx != "" {
			parts = append(parts, "### 对话历史\n"+wctx)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}

// rankWithDecay 组合评分重排：cosine_similarity × decay^(days/30)，取 TopK。
func rankWithDecay(msgs []model.ChatMessage, distances []float64, decayFactor float64, limit int) []model.ChatMessage {
	type scored struct {
		msg   model.ChatMessage
		score float64
	}
	now := time.Now()
	scoredList := make([]scored, 0, len(msgs))
	for i, msg := range msgs {
		simScore := 1.0 - distances[i]
		if simScore < 0 {
			simScore = 0
		}
		daysOld := now.Sub(msg.CreatedAt).Hours() / 24
		timeWeight := 1.0
		if decayFactor > 0 && daysOld > 0 {
			timeWeight = math.Pow(decayFactor, daysOld/30)
		}
		scoredList = append(scoredList, scored{msg, simScore * timeWeight})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	result := make([]model.ChatMessage, 0, limit)
	for i := 0; i < len(scoredList) && len(result) < limit; i++ {
		result = append(result, scoredList[i].msg)
	}
	return result
}

// AddSemantic 写入语义记忆（外部触发）
func (m *MemoryManager) AddSemantic(ctx context.Context, content string, importance float64, category string) error {
	return m.semantic.Add(ctx, &MemoryItem{
		Content:    content,
		Importance: importance,
	})
}
