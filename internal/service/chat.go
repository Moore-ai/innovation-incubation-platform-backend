package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/errcode"
)

type ChatService struct {
	engine *agent.Engine
	repo   *repository.ChatRepo
	memory *agentmemory.MemoryManager
	ai     *aiclient.Client
	cfg    config.AgentConfig
}

func NewChatService(engine *agent.Engine, repo *repository.ChatRepo, memory *agentmemory.MemoryManager, ai *aiclient.Client, cfg config.AgentConfig) *ChatService {
	return &ChatService{engine: engine, repo: repo, memory: memory, ai: ai, cfg: cfg}
}

// CreateSession 创建新会话
func (s *ChatService) CreateSession(userID uint, title string) (*model.ChatSession, error) {
	sess := &model.ChatSession{
		UserID:        userID,
		Title:         title,
		LastMessageAt: time.Now(),
		MessageCount:  0,
	}
	if err := s.repo.CreateSession(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// ListSessions 获取会话列表
func (s *ChatService) ListSessions(userID uint) ([]model.ChatSession, error) {
	return s.repo.ListSessionsByUser(userID)
}

// GetSession 获取会话详情（含消息列表）
func (s *ChatService) GetSession(sessionID uint, userID uint) (*model.ChatSession, []model.ChatMessage, error) {
	sess, err := s.repo.FindSessionByID(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if sess.UserID != userID {
		return nil, nil, err
	}
	msgs, err := s.repo.ListMessagesBySession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	return sess, msgs, nil
}

// DeleteSession 删除会话（软删除）
func (s *ChatService) DeleteSession(sessionID uint, userID uint) error {
	sess, err := s.repo.FindSessionByID(sessionID)
	if err != nil {
		return err
	}
	if sess.UserID != userID {
		return err
	}
	return s.repo.DeleteSession(sessionID)
}

// Run 执行对话，返回 RunResult + SSE 事件通过回调推送
func (s *ChatService) Run(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(agent.SSEEvent)) (*agent.RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.RequestTimeoutSec)*time.Second)
	defer cancel()

	result, err := s.engine.Run(ctx, sessionID, userMessage, role, onEvent)
	if err == nil && result.ReflectTrigger {
		go s.writeSemantic(agent.UserIDFromCtx(ctx), result.Messages)
	}
	return result, err
}

// SaveMessages 持久化消息并更新会话统计
func (s *ChatService) SaveMessages(sessionID uint, userID uint, records []agent.ChatMessageRecord) error {
	msgs := make([]model.ChatMessage, 0, len(records))
	for _, r := range records {
		msgs = append(msgs, model.ChatMessage{
			SessionID:  sessionID,
			UserID:     userID,
			Role:       r.Role,
			Content:    r.Content,
			ToolCallID: r.ToolCallID,
			ToolCalls:  r.ToolCalls,
		})
	}
	if err := s.repo.CreateMessages(msgs); err != nil {
		return err
	}
	return s.repo.UpdateSessionStats(sessionID, time.Now(), len(msgs))
}

// EditAndResend 编辑最后一条用户消息并重新发送。
// 先执行 Agent，成功后在事务中软删旧消息、插入新消息。
func (s *ChatService) EditAndResend(ctx context.Context, sessionID uint, messageID uint, userMessage, role string, onEvent func(agent.SSEEvent)) (*agent.RunResult, error) {
	// 1. 查 session 下最后一条 user 消息
	msgs, err := s.repo.ListMessagesBySession(sessionID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("会话不存在")
	}
	if len(msgs) == 0 {
		return nil, errcode.ErrInvalidParams.WithMsg("会话为空")
	}

	var lastUserMsg *model.ChatMessage
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUserMsg = &msgs[i]
			break
		}
	}
	if lastUserMsg == nil {
		return nil, errcode.ErrInvalidParams.WithMsg("会话无用户消息")
	}
	if lastUserMsg.ID != messageID {
		return nil, errcode.ErrInvalidParams.WithMsg("只能编辑最后一条用户消息")
	}

	// 请求级超时前注入排除标记，让记忆加载跳过即将被替换的消息。
	ctx = agent.WithExcludeFromMessageID(ctx, messageID)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.RequestTimeoutSec)*time.Second)
	defer cancel()

	// 3. 执行 Agent（不修改数据库）
	result, err := s.engine.Run(ctx, sessionID, userMessage, role, onEvent)
	if err != nil {
		return result, err
	}

	// 4. 组装新记录
	newRecords := append(
		[]agent.ChatMessageRecord{{Role: "user", Content: userMessage}},
		result.Messages...,
	)
	newModels := make([]model.ChatMessage, 0, len(newRecords))
	for _, r := range newRecords {
		newModels = append(newModels, model.ChatMessage{
			SessionID:  sessionID,
			UserID:     agent.UserIDFromCtx(ctx),
			Role:       r.Role,
			Content:    r.Content,
			ToolCallID: r.ToolCallID,
			ToolCalls:  r.ToolCalls,
		})
	}

	// 5. 事务内软删旧消息 + 插入新消息
	var deletedCount int64
	deletedCount, err = s.repo.ReplaceMessages(sessionID, messageID, newModels)
	if err != nil {
		slog.Error("替换消息事务失败", "error", err, "session_id", sessionID)
		return result, errcode.ErrInternal.WithMsg("替换消息失败")
	}
		// 6. 更新会话统计
	delta := len(newRecords) - int(deletedCount)
	if err := s.repo.UpdateSessionStats(sessionID, time.Now(), delta); err != nil {
		slog.Error("更新会话统计失败", "error", err, "session_id", sessionID)
	}

	return result, nil
}

// AddSemanticMemory 写入语义记忆（Reflect 触发后调用）
func (s *ChatService) AddSemanticMemory(ctx context.Context, userID uint, content string, category agentmemory.Category) error {
	return s.memory.AddSemantic(ctx, userID, content, 0.5, category)
}

// writeSemantic 用 LLM 分析对话上下文，同时提炼操作教训和用户偏好写入语义记忆。
func (s *ChatService) writeSemantic(userID uint, msgs []agent.ChatMessageRecord) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 构建对话上下文：工具调用 + 工具返回 + 用户消息
	var b strings.Builder
	type toolCallInfo struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	for _, m := range msgs {
		switch m.Role {
		case "user":
			fmt.Fprintf(&b, "用户: %s\n", m.Content)
		case "assistant":
			if m.ToolCalls != "" {
				var calls []toolCallInfo
				if json.Unmarshal([]byte(m.ToolCalls), &calls) == nil {
					for _, call := range calls {
						fmt.Fprintf(&b, "调用工具: %s, 参数: %s\n", call.Function.Name, call.Function.Arguments)
					}
				}
			}
		case "tool":
			fmt.Fprintf(&b, "工具返回: %s\n", m.Content)
		}
	}
	contextStr := b.String()
	if contextStr == "" {
		return
	}

	resp, err := s.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: s.ai.Model(),
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: `你是一个AI学习助手。分析以下对话，输出 JSON：

{
  "lessons": [],
  "preferences": []
}

规则：
- lessons: 工具调用失败的教训。每条一句话，包含"哪个工具失败了、原因、如何避免"。无失败则留空数组。
- preferences: 用户明确表达的偏好或反馈。如"更喜欢饼图"、"回复简洁些"、"默认用PDF"。无偏好则留空数组。
- 每条不超过80字，数组最多3条。
- 严格输出JSON，不要其他内容。`},
			{Role: openai.ChatMessageRoleUser, Content: contextStr},
		},
	})
	if err != nil || len(resp.Choices) == 0 {
		slog.Warn("语义提炼失败", "error", err)
		return
	}

	raw := CleanLLMOutput(resp.Choices[0].Message.Content)

	var result struct {
		Lessons     []string `json:"lessons"`
		Preferences []string `json:"preferences"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		slog.Warn("语义提炼 JSON 解析失败", "error", err, "raw", raw[:min(len(raw), 200)])
		return
	}

	for _, lesson := range result.Lessons {
		lesson = strings.TrimSpace(lesson)
		if lesson != "" {
			if err := s.AddSemanticMemory(ctx, userID, lesson, agentmemory.CategoryLesson); err != nil {
				slog.Error("写入教训失败", "error", err)
			}
		}
	}
	for _, pref := range result.Preferences {
		pref = strings.TrimSpace(pref)
		if pref != "" {
			if err := s.AddSemanticMemory(ctx, userID, pref, agentmemory.CategoryPreference); err != nil {
				slog.Error("写入偏好失败", "error", err)
			}
		}
	}
}
