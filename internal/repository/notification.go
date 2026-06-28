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

func (r *NotificationRepo) ListByUser(userID uint, page, pageSize int) ([]model.Notification, int64, error) {
	var list []model.Notification
	var total int64
	if err := r.db.Model(&model.Notification{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
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
