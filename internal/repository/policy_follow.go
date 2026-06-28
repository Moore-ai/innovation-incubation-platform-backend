package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type PolicyFollowRepo struct {
	db *gorm.DB
}

func NewPolicyFollowRepo(db *gorm.DB) *PolicyFollowRepo {
	return &PolicyFollowRepo{db: db}
}

func (r *PolicyFollowRepo) Create(entID, policyID uint) error {
	return r.db.Create(&model.PolicyFollow{EnterpriseID: entID, PolicyID: policyID}).Error
}

func (r *PolicyFollowRepo) Delete(entID, policyID uint) error {
	return r.db.Where("enterprise_id = ? AND policy_id = ?", entID, policyID).Delete(&model.PolicyFollow{}).Error
}

func (r *PolicyFollowRepo) Exists(entID, policyID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.PolicyFollow{}).Where("enterprise_id = ? AND policy_id = ?", entID, policyID).Count(&count).Error
	return count > 0, err
}

func (r *PolicyFollowRepo) ListByEnterprise(entID uint, page, pageSize int) ([]model.PolicyFollow, int64, error) {
	var list []model.PolicyFollow
	var total int64
	q := r.db.Model(&model.PolicyFollow{}).Where("enterprise_id = ?", entID)
	q.Count(&total)
	err := q.Preload("Policy").Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *PolicyFollowRepo) FindEnterpriseIDsByPolicy(policyID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.PolicyFollow{}).Where("policy_id = ?", policyID).Pluck("enterprise_id", &ids).Error
	return ids, err
}
