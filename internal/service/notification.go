package service

import (
	"log/slog"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
)

type NotificationService struct {
	repo *repository.NotificationRepo
	hub  *SSEHub
}

func NewNotificationService(repo *repository.NotificationRepo, hub *SSEHub) *NotificationService {
	return &NotificationService{repo: repo, hub: hub}
}

func (s *NotificationService) Send(userID uint, ntype model.NotificationType, title, content string, targetType model.TargetType, targetID uint) error {
	n := &model.Notification{
		UserID:     userID,
		Type:       ntype,
		Title:      title,
		Content:    content,
		TargetType: targetType,
		TargetID:   targetID,
	}
	if err := s.repo.Create(n); err != nil {
		slog.Warn("notification create failed", "user_id", userID, "type", ntype, "error", err)
		return err
	}
	s.hub.Notify(userID, SSEEvent{
		ID:         n.ID,
		CreatedAt:  n.CreatedAt.UnixMilli(),
		Type:       ntype,
		Title:      title,
		Content:    content,
		TargetType: targetType,
		TargetID:   targetID,
	})
	return nil
}
