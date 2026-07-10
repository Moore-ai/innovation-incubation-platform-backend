package repository

import (
	"innovation-incubation-platform-backend/internal/model"
)

func (r *ChatRepo) CreateMessages(msgs []model.ChatMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	normalizeChatMessageJSONFields(msgs)
	return r.db.Create(&msgs).Error
}

func (r *ChatRepo) ListMessagesBySession(sessionID uint) ([]model.ChatMessage, error) {
	var msgs []model.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&msgs).Error
	return msgs, err
}

// LoadMessagesPage 游标分页加载消息（按 created_at DESC），返回 (消息, 下一页 cursor, 是否有更多, error)
func (r *ChatRepo) LoadMessagesPage(sessionID uint, cursorID uint, limit int) ([]model.ChatMessage, uint, bool, error) {
	var msgs []model.ChatMessage
	q := r.db.Where("session_id = ?", sessionID).Order("created_at DESC, id DESC").Limit(limit + 1)
	if cursorID > 0 {
		q = q.Where("id < ?", cursorID)
	}
	if err := q.Find(&msgs).Error; err != nil {
		return nil, 0, false, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	var nextCursor uint
	if hasMore {
		nextCursor = msgs[len(msgs)-1].ID
	}
	return msgs, nextCursor, hasMore, nil
}
