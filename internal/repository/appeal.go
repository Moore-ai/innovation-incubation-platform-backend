package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type AppealRepo struct {
	db *gorm.DB
}

func NewAppealRepo(db *gorm.DB) *AppealRepo {
	return &AppealRepo{db: db}
}

func (r *AppealRepo) Create(appeal *model.Appeal) error {
	return r.db.Create(appeal).Error
}

func (r *AppealRepo) FindByID(id uint) (*model.Appeal, error) {
	var a model.Appeal
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AppealRepo) ListBySubmitter(submitterID uint, page, pageSize int) ([]model.Appeal, int64, error) {
	var appeals []model.Appeal
	var total int64
	q := r.db.Model(&model.Appeal{}).Where("submitted_by = ?", submitterID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&appeals).Error
	return appeals, total, err
}

func (r *AppealRepo) ListAll(status string, problemType string, page, pageSize int) ([]model.Appeal, int64, error) {
	var appeals []model.Appeal
	var total int64
	q := r.db.Model(&model.Appeal{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if problemType != "" {
		q = q.Where("problem_type = ?", problemType)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&appeals).Error
	return appeals, total, err
}

func (r *AppealRepo) UpdateStatus(id uint, status model.AppealStatus) error {
	return r.db.Model(&model.Appeal{}).Where("id = ?", id).Update("status", status).Error
}
