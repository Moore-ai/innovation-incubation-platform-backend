package repository

import (
	"fmt"

	"innovation-incubation-platform-backend/internal/model"
)

// SearchMessages 情景记忆：全文搜索历史消息
func (r *ChatRepo) SearchMessages(userID uint, query string, limit int) ([]model.ChatMessage, error) {
	var msgs []model.ChatMessage
	err := r.db.Where("user_id = ?", userID).
		Where("to_tsvector('simple', content) @@ plainto_tsquery('simple', ?)", query).
		Order("created_at DESC").Limit(limit).Find(&msgs).Error
	return msgs, err
}

// SearchMessagesByVector 向量语义相似度检索历史消息。
func (r *ChatRepo) SearchMessagesByVector(userID uint, embedding []float32, limit int) ([]model.ChatMessage, error) {
	msgs, _, err := r.SearchMessagesByVectorWithDistance(userID, embedding, limit)
	return msgs, err
}

// messageWithDistance 临时结构体，用于带距离值的向量检索。
type messageWithDistance struct {
	model.ChatMessage
	Distance float64 `gorm:"column:distance"`
}

// SearchMessagesByVectorWithDistance 向量检索历史消息，返回消息和对应的余弦距离值。
func (r *ChatRepo) SearchMessagesByVectorWithDistance(userID uint, embedding []float32, limit int) ([]model.ChatMessage, []float64, error) {
	vecStr := model.PGVector(embedding).String()
	var rows []messageWithDistance
	err := r.db.Table("chat_messages").
		Select("*, embedding <=> '"+vecStr+"' AS distance").
		Where("user_id = ? AND embedding IS NOT NULL", userID).
		Order(fmt.Sprintf("embedding <=> '%s'", vecStr)).
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, nil, err
	}
	msgs := make([]model.ChatMessage, len(rows))
	dists := make([]float64, len(rows))
	for i, row := range rows {
		msgs[i] = row.ChatMessage
		dists[i] = row.Distance
	}
	return msgs, dists, nil
}
