package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type DeletionRepo struct {
	db *gorm.DB
}

func NewDeletionRepo(db *gorm.DB) *DeletionRepo {
	return &DeletionRepo{db: db}
}

func (r *DeletionRepo) Create(req *model.AccountDeletionRequest) error {
	return r.db.Create(req).Error
}

func (r *DeletionRepo) FindByID(id uint) (*model.AccountDeletionRequest, error) {
	var req model.AccountDeletionRequest
	err := r.db.First(&req, id).Error
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *DeletionRepo) ListPending(page, pageSize int) ([]model.AccountDeletionRequest, int64, error) {
	var list []model.AccountDeletionRequest
	var total int64
	q := r.db.Model(&model.AccountDeletionRequest{}).Where("status = ?", model.ApprovalPending)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *DeletionRepo) UpdateStatus(id uint, status model.ApprovalStatus, reviewerID uint, comment string) error {
	return r.db.Model(&model.AccountDeletionRequest{}).Where("id = ?", id).
		Updates(map[string]any{
			"status":         status,
			"reviewer_id":    reviewerID,
			"review_comment": comment,
		}).Error
}

// FindUserIDByEnterpriseID 通过企业 ID 查找用户 ID
func (r *DeletionRepo) FindUserIDByEnterpriseID(entID uint) (uint, error) {
	var ent model.Enterprise
	err := r.db.Select("user_id").First(&ent, entID).Error
	if err != nil {
		return 0, err
	}
	return ent.UserID, nil
}
