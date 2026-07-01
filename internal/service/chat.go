package service

import (
	"context"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
)

type ChatService struct {
	engine *agent.Engine
	repo   *repository.ChatRepo
	memory *agentmemory.MemoryManager
	cfg    config.AgentConfig
}

func NewChatService(engine *agent.Engine, repo *repository.ChatRepo, memory *agentmemory.MemoryManager, cfg config.AgentConfig) *ChatService {
	return &ChatService{engine: engine, repo: repo, memory: memory, cfg: cfg}
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
	// 请求级超时
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.RequestTimeoutSec)*time.Second)
	defer cancel()

	return s.engine.Run(ctx, sessionID, userMessage, role, onEvent)
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

// AddSemanticMemory 写入语义记忆（Reflect 触发后调用）
func (s *ChatService) AddSemanticMemory(ctx context.Context, content string) error {
	return s.memory.AddSemantic(ctx, content, 0.5, "lesson")
}
