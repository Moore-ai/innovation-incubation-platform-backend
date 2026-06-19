package repository

import (
	"innovation-incubation-platform-backend/internal/model"
	"gorm.io/gorm"
)

type CommonRepo struct {
	db *gorm.DB
}

func NewCommonRepo(db *gorm.DB) *CommonRepo {
	return &CommonRepo{db: db}
}

func (r *CommonRepo) ListPoliciesByTarget(role string, page, pageSize int) ([]model.Policy, int64, error) {
	var policies []model.Policy
	var total int64
	q := r.db.Model(&model.Policy{}).
		Joins("JOIN policy_templates ON policy_templates.id = policies.template_id").
		Where("policies.status = ? AND (policy_templates.target_role = ? OR policy_templates.target_role = 'both')", model.PolicyPublished, role)
	q.Count(&total)
	err := q.Preload("Template").Order("policies.created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&policies).Error
	return policies, total, err
}

func (r *CommonRepo) FindPolicyByID(id uint) (*model.Policy, error) {
	var policy model.Policy
	err := r.db.Preload("Template").First(&policy, id).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *CommonRepo) CreatePolicyApplication(app *model.PolicyApplication) error {
	return r.db.Create(app).Error
}

func (r *CommonRepo) ListApplicationsByApplicant(applicantType string, applicantID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	var apps []model.PolicyApplication
	var total int64
	q := r.db.Model(&model.PolicyApplication{}).
		Where("applicant_type = ? AND applicant_id = ?", applicantType, applicantID)
	q.Count(&total)
	err := q.Preload("Policy").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}
