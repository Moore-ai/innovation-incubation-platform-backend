package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type NotificationController struct {
	repo *repository.NotificationRepo
	hub  *service.SSEHub
}

func NewNotificationController(repo *repository.NotificationRepo, hub *service.SSEHub) *NotificationController {
	return &NotificationController{repo: repo, hub: hub}
}

func (ctl *NotificationController) Subscribe(c *gin.Context) {
	userID := middleware.GetUserID(c)

	ch, err := ctl.hub.Subscribe(userID)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("连接数超过限制"))
		return
	}
	defer ctl.hub.Unsubscribe(userID, ch)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.WriteHeader(200)
	c.Writer.Flush()

	recent, _ := ctl.repo.FindRecentByUser(userID, 20)
	b, _ := json.Marshal(recent)
	fmt.Fprintf(c.Writer, "event: init\ndata: %s\n\n", string(b))
	c.Writer.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-ch:
			b, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "event: update\ndata: %s\n\n", string(b))
			c.Writer.Flush()
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}
