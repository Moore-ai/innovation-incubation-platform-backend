package repository

import (
	"fmt"

	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type ChatRepo struct {
	db *gorm.DB
}

func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

// --- ChatSession ---

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

// --- ChatMessage ---

func (r *ChatRepo) CreateMessages(msgs []model.ChatMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	return r.db.Create(&msgs).Error
}

func (r *ChatRepo) ListMessagesBySession(sessionID uint) ([]model.ChatMessage, error) {
	var msgs []model.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Find(&msgs).Error
	return msgs, err
}

// SearchMessages 情景记忆：全文搜索历史消息
func (r *ChatRepo) SearchMessages(userID uint, query string, limit int) ([]model.ChatMessage, error) {
	var msgs []model.ChatMessage
	err := r.db.Where("user_id = ?", userID).
		Where("to_tsvector('simple', content) @@ plainto_tsquery('simple', ?)", query).
		Order("created_at DESC").Limit(limit).Find(&msgs).Error
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

// --- SemanticMemory ---

func (r *ChatRepo) CreateSemanticMemory(m *model.SemanticMemory) error {
	return r.db.Create(m).Error
}

func (r *ChatRepo) RetrieveSemanticByCategory(userID uint, categories []string, keyword string, limit int) ([]model.SemanticMemory, error) {
	var ms []model.SemanticMemory
	q := r.db.Where("(user_id IS NULL OR user_id = ?)", userID).
		Where("category IN ?", categories)
	if keyword != "" {
		q = q.Where("content ILIKE ?", "%"+keyword+"%")
	}
	err := q.Order("importance DESC").Limit(limit).Find(&ms).Error
	return ms, err
}

// RetrieveSemanticByVector 语义记忆向量检索（仅当 embeddingClient 可用时调用）
func (r *ChatRepo) RetrieveSemanticByVector(userID uint, embedding []float32, limit int) ([]model.SemanticMemory, error) {
	var ms []model.SemanticMemory
	vecStr := model.PGVector(embedding).String()
	err := r.db.Where("(user_id IS NULL OR user_id = ?)", userID).
		Order(fmt.Sprintf("embedding <=> '%s'", vecStr)).
		Limit(limit).Find(&ms).Error
	return ms, err
}
