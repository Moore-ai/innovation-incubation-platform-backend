package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type NotificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(n *model.Notification) error {
	return r.db.Create(n).Error
}

func (r *NotificationRepo) FindRecentByUser(userID uint, limit int) ([]model.Notification, error) {
	var list []model.Notification
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&list).Error
	return list, err
}

func (r *NotificationRepo) MarkAsRead(ids []uint, userID uint) error {
	return r.db.Model(&model.Notification{}).
		Where("id IN ? AND user_id = ?", ids, userID).
		Update("is_read", true).Error
}
