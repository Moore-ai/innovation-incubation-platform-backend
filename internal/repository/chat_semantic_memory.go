package repository

import (
	"fmt"

	"innovation-incubation-platform-backend/internal/model"
)

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
