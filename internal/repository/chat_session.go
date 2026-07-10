package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

func (r *ChatRepo) CreateSession(s *model.ChatSession) error {
	return r.db.Create(s).Error
}

func (r *ChatRepo) FindSessionByID(id uint) (*model.ChatSession, error) {
	var s model.ChatSession
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ChatRepo) ListSessionsByUser(userID uint) ([]model.ChatSession, error) {
	var sessions []model.ChatSession
	err := r.db.Where("user_id = ?", userID).Order("last_message_at DESC").Find(&sessions).Error
	return sessions, err
}

func (r *ChatRepo) UpdateSessionStats(sessionID uint, lastMessageAt any, delta int) error {
	return r.db.Model(&model.ChatSession{}).Where("id = ?", sessionID).
		Updates(map[string]any{"last_message_at": lastMessageAt, "message_count": gorm.Expr("message_count + ?", delta)}).Error
}

func (r *ChatRepo) DeleteSession(id uint) error {
	return r.db.Delete(&model.ChatSession{}, id).Error
}
