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
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("enterprise_id = ? AND policy_id = ?", entID, policyID).
			Delete(&model.PolicyFollow{}).Error; err != nil {
			return err
		}
		return tx.Create(&model.PolicyFollow{EnterpriseID: entID, PolicyID: policyID}).Error
	})
}

func (r *PolicyFollowRepo) Delete(entID, policyID uint) error {
	return r.db.Unscoped().Where("enterprise_id = ? AND policy_id = ?", entID, policyID).Delete(&model.PolicyFollow{}).Error
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
	err := q.Preload("Policy").Order("created_at ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *PolicyFollowRepo) ListPoliciesByEnterprise(entID uint, page, pageSize int) ([]model.Policy, int64, error) {
	var policies []model.Policy
	var total int64
	q := r.db.Model(&model.Policy{}).
		Joins("JOIN policy_follows ON policy_follows.policy_id = policies.id").
		Where("policy_follows.enterprise_id = ? AND policy_follows.deleted_at IS NULL", entID)
	q.Count(&total)
	err := q.Order("policy_follows.created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&policies).Error
	for i := range policies {
		policies[i].Followed = true
	}
	return policies, total, err
}

func (r *PolicyFollowRepo) FindPolicyIDsByEnterprise(entID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.PolicyFollow{}).Where("enterprise_id = ?", entID).Pluck("policy_id", &ids).Error
	return ids, err
}

func (r *PolicyFollowRepo) FindEnterpriseIDsByPolicy(policyID uint) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.PolicyFollow{}).Where("policy_id = ?", policyID).Pluck("enterprise_id", &ids).Error
	return ids, err
}
