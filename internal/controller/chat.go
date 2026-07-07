package controller

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/service"
	agent "innovation-incubation-platform-backend/internal/service/agent"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type ChatController struct {
	svc *service.ChatService
	cfg *config.Config
}

func NewChatController(svc *service.ChatService, cfg *config.Config) *ChatController {
	return &ChatController{svc: svc, cfg: cfg}
}

func (ctl *ChatController) publicAIError(err error) string {
	if ctl.cfg != nil && strings.TrimSpace(ctl.cfg.AI.OpenAI.APIKey) == "" {
		return "AI 服务未配置，请联系管理员检查模型 API Key。"
	}
	raw := strings.ToLower(err.Error())
	if strings.Contains(raw, "401") ||
		strings.Contains(raw, "unauthorized") ||
		strings.Contains(raw, "authentication") ||
		strings.Contains(raw, "api key") {
		return "AI 服务鉴权失败，请联系管理员检查模型 API Key。"
	}
	if strings.Contains(raw, "timeout") || strings.Contains(raw, "deadline") {
		return "AI 服务响应超时，请稍后重试。"
	}
	return "AI 服务暂不可用，请稍后重试。"
}

func (ctl *ChatController) CreateSession(c *gin.Context) {
	var req dto.CreateChatSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	userID := middleware.GetUserID(c)
	sess, err := ctl.svc.CreateSession(userID, req.Title)
	if err != nil {
		response.Error(c, errcode.ErrInternal.WithMsg("创建会话失败"))
		return
	}
	response.Created(c, sess, fmt.Sprintf("/api/v1/chat/sessions/%d", sess.ID))
}

func (ctl *ChatController) ListSessions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	sessions, err := ctl.svc.ListSessions(userID)
	if err != nil {
		response.Error(c, errcode.ErrInternal.WithMsg("获取会话列表失败"))
		return
	}
	response.Success(c, sessions)
}

func (ctl *ChatController) GetSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var uri struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	sess, msgs, err := ctl.svc.GetSession(uri.ID, userID)
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("会话不存在"))
		return
	}
	response.Success(c, gin.H{"session": sess, "messages": msgs})
}

func (ctl *ChatController) DeleteSession(c *gin.Context) {
	userID := middleware.GetUserID(c)
	var uri struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if err := ctl.svc.DeleteSession(uri.ID, userID); err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("会话不存在"))
		return
	}
	response.Success(c, nil)
}

// EditAndResend 编辑最后一条用户消息并重新发送（SSE）。
func (ctl *ChatController) EditAndResend(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)

	var uri struct {
		SessionID uint `uri:"id" binding:"required"`
		MessageID uint `uri:"messageId" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}

	var req dto.SendChatMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}

	if ctl.cfg.Agent.MessageMaxChars > 0 && len([]rune(req.Content)) > ctl.cfg.Agent.MessageMaxChars {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("消息超过最大长度限制"))
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrInternal.WithMsg("不支持 SSE"))
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	ctx := c.Request.Context()
	ctx = agent.WithUserID(ctx, userID)
	ctx = agent.WithRole(ctx, role)
	if len(req.State) > 0 {
		ctx = agent.WithState(ctx, req.State)
	}

	_, err := ctl.svc.EditAndResend(ctx, uri.SessionID, uri.MessageID, req.Content, role, func(evt agent.SSEEvent) {
		agent.WriteSSEEvent(c.Writer, flusher, evt)
	})

	if err != nil {
		agent.WriteSSEEvent(c.Writer, flusher, agent.SSEEvent{Type: "error", Data: map[string]string{"message": ctl.publicAIError(err)}})
		return
	}
}

// SendMessage SSE 流式响应
func (ctl *ChatController) SendMessage(c *gin.Context) {
	userID := middleware.GetUserID(c)
	role := middleware.GetRole(c)

	var uri struct {
		ID uint `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&uri); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}

	var req dto.SendChatMessageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}

	if ctl.cfg.Agent.MessageMaxChars > 0 && len([]rune(req.Content)) > ctl.cfg.Agent.MessageMaxChars {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("消息超过最大长度限制"))
		return
	}

	_, _, err := ctl.svc.GetSession(uri.ID, userID)
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("会话不存在"))
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrInternal.WithMsg("不支持 SSE"))
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	ctx := c.Request.Context()
	ctx = agent.WithUserID(ctx, userID)
	ctx = agent.WithRole(ctx, role)
	if len(req.State) > 0 {
		ctx = agent.WithState(ctx, req.State)
	}

	result, err := ctl.svc.Run(ctx, uri.ID, req.Content, role, func(evt agent.SSEEvent) {
		agent.WriteSSEEvent(c.Writer, flusher, evt)
	})

	if err != nil {
		agent.WriteSSEEvent(c.Writer, flusher, agent.SSEEvent{Type: "error", Data: map[string]string{"message": ctl.publicAIError(err)}})
		return
	}

	// 持久化消息（追加用户消息）
	userRecord := agent.ChatMessageRecord{
		Role:    "user",
		Content: req.Content,
	}
	allRecords := append([]agent.ChatMessageRecord{userRecord}, result.Messages...)
	go func(sessionID, userID uint, msgs []agent.ChatMessageRecord) {
		if err := ctl.svc.SaveMessages(sessionID, userID, msgs); err != nil {
			slog.Error("保存聊天消息失败", "error", err, "session_id", sessionID)
		}
	}(uri.ID, userID, allRecords)
}
