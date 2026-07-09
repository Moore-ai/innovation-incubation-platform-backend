package service

import (
	"context"
	"log/slog"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/errcode"
)

type ChatService struct {
	engine *agent.Engine
	repo   *repository.ChatRepo
	ai     *aiclient.Client
	cfg    config.AgentConfig
}

func NewChatService(engine *agent.Engine, repo *repository.ChatRepo, ai *aiclient.Client, cfg config.AgentConfig) *ChatService {
	return &ChatService{engine: engine, repo: repo, ai: ai, cfg: cfg}
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

