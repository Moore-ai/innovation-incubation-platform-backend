package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type GovernmentRepo struct {
	db *gorm.DB
}

func NewGovernmentRepo(db *gorm.DB) *GovernmentRepo {
	return &GovernmentRepo{db: db}
}

func (r *GovernmentRepo) CreatePolicy(p *model.Policy) error {
	return r.db.Create(p).Error
}

func (r *GovernmentRepo) FindPolicyByID(id uint) (*model.Policy, error) {
	var p model.Policy
	err := r.db.First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GovernmentRepo) UpdatePolicy(p *model.Policy) error {
	return r.db.Save(p).Error
}

func (r *GovernmentRepo) DeletePolicy(id uint) error {
	return r.db.Delete(&model.Policy{}, id).Error
}

func (r *GovernmentRepo) ListPolicies(page, pageSize int) ([]model.Policy, int64, error) {
	var policies []model.Policy
	var total int64
	q := r.db.Model(&model.Policy{})
	q.Count(&total)
	err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&policies).Error
	return policies, total, err
}

func (r *GovernmentRepo) SearchEnterprises(keyword string, page, pageSize int) ([]model.Enterprise, int64, error) {
	var ents []model.Enterprise
	var total int64
	like := "%" + keyword + "%"
	q := r.db.Model(&model.Enterprise{}).
		Where("name LIKE ? OR credit_code LIKE ? OR industry LIKE ?", like, like, like)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&ents).Error
	return ents, total, err
}

func (r *GovernmentRepo) FindEnterpriseByID(id uint) (*model.Enterprise, error) {
	var ent model.Enterprise
	err := r.db.First(&ent, id).Error
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

func (r *GovernmentRepo) UpdateEnterprise(ent *model.Enterprise) error {
	return r.db.Save(ent).Error
}

func (r *GovernmentRepo) SearchCarriers(keyword string, page, pageSize int) ([]model.Carrier, int64, error) {
	var carriers []model.Carrier
	var total int64
	like := "%" + keyword + "%"
	q := r.db.Model(&model.Carrier{}).Where("name LIKE ? OR address LIKE ?", like, like)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&carriers).Error
	return carriers, total, err
}

func (r *GovernmentRepo) FindCarrierByID(id uint) (*model.Carrier, error) {
	var carrier model.Carrier
	err := r.db.First(&carrier, id).Error
	if err != nil {
		return nil, err
	}
	return &carrier, nil
}

func (r *GovernmentRepo) ListPolicyApplicationsForReview(page, pageSize int) ([]model.PolicyApplication, int64, error) {
	var apps []model.PolicyApplication
	var total int64
	q := r.db.Model(&model.PolicyApplication{}).Where("status = 'pending' OR status = 'gov_review'")
	q.Count(&total)
	err := q.Preload("Policy").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}

func (r *GovernmentRepo) UpdateApplicationStatus(id uint, status string) error {
	return r.db.Model(&model.PolicyApplication{}).Where("id = ?", id).Update("status", status).Error
}

func (r *GovernmentRepo) CreatePerformanceTemplate(t *model.PerformanceTemplate) error {
	return r.db.Create(t).Error
}

func (r *GovernmentRepo) CreatePerformanceCampaign(c *model.PerformanceCampaign) error {
	return r.db.Create(c).Error
}

func (r *GovernmentRepo) ListPerformanceSubmissions(page, pageSize int) ([]model.PerformanceSubmission, int64, error) {
	var subs []model.PerformanceSubmission
	var total int64
	q := r.db.Model(&model.PerformanceSubmission{})
	q.Count(&total)
	err := q.Preload("Campaign").Preload("Carrier").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&subs).Error
	return subs, total, err
}

func (r *GovernmentRepo) FindPerformanceSubmission(id uint) (*model.PerformanceSubmission, error) {
	var sub model.PerformanceSubmission
	err := r.db.First(&sub, id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *GovernmentRepo) FindUserIDsByRole(role string) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.User{}).Where("role = ?", role).Pluck("id", &ids).Error
	return ids, err
}

func (r *GovernmentRepo) UpdateSubmissionScore(id uint, status string, score float64) error {
	return r.db.Model(&model.PerformanceSubmission{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "score": score}).Error
}
