package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

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

	// 校验消息长度
	if ctl.cfg.Agent.MessageMaxChars > 0 && len([]rune(req.Content)) > ctl.cfg.Agent.MessageMaxChars {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("消息超过最大长度限制"))
		return
	}

	// SSE headers
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

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrInternal.WithMsg("不支持 SSE"))
		return
	}

	_, err := ctl.svc.EditAndResend(ctx, uri.SessionID, uri.MessageID, req.Content, role, func(evt agent.SSEEvent) {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(agent.SSEEvent{Type: "error", Data: map[string]string{"message": err.Error()}})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
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

	// 校验消息长度
	if ctl.cfg.Agent.MessageMaxChars > 0 && len([]rune(req.Content)) > ctl.cfg.Agent.MessageMaxChars {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("消息超过最大长度限制"))
		return
	}

	// 校验会话归属
	_, _, err := ctl.svc.GetSession(uri.ID, userID)
	if err != nil {
		response.Error(c, errcode.ErrNotFound.WithMsg("会话不存在"))
		return
	}

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	ctx := c.Request.Context()
	// 注入 user_id、role 和 state 到 context
	ctx = agent.WithUserID(ctx, userID)
	ctx = agent.WithRole(ctx, role)
	if len(req.State) > 0 {
		ctx = agent.WithState(ctx, req.State)
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, errcode.ErrInternal.WithMsg("不支持 SSE"))
		return
	}

	result, err := ctl.svc.Run(ctx, uri.ID, req.Content, role, func(evt agent.SSEEvent) {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(agent.SSEEvent{Type: "error", Data: map[string]string{"message": err.Error()}})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
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
