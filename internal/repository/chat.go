package repository

import (
	"strings"

	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type ChatRepo struct {
	db *gorm.DB
}

func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

func normalizeChatMessageJSONFields(msgs []model.ChatMessage) {
	for i := range msgs {
		if strings.TrimSpace(msgs[i].ToolCalls) == "" {
			msgs[i].ToolCalls = "[]"
		}
	}
}

// ReplaceMessages 在事务内软删旧消息并插入新消息，返回软删条数。
func (r *ChatRepo) ReplaceMessages(sessionID, fromMessageID uint, newMsgs []model.ChatMessage) (int64, error) {
	var deletedCount int64
	normalizeChatMessageJSONFields(newMsgs)
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("session_id = ? AND id >= ?", sessionID, fromMessageID).
			Delete(&model.ChatMessage{})
		if result.Error != nil {
			return result.Error
		}
		deletedCount = result.RowsAffected
		return tx.Create(&newMsgs).Error
	})
	return deletedCount, err
}
