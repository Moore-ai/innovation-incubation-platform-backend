package repository

import (
	"innovation-incubation-platform-backend/internal/model"
	"gorm.io/gorm"
)

type CarrierRepo struct {
	db *gorm.DB
}

func NewCarrierRepo(db *gorm.DB) *CarrierRepo {
	return &CarrierRepo{db: db}
}

func (r *CarrierRepo) FindCarrierByUserID(userID uint) (*model.Carrier, error) {
	var carrier model.Carrier
	err := r.db.Where("user_id = ?", userID).First(&carrier).Error
	if err != nil {
		return nil, err
	}
	return &carrier, nil
}

func (r *CarrierRepo) UpdateCarrier(carrier *model.Carrier) error {
	return r.db.Save(carrier).Error
}

func (r *CarrierRepo) ListPendingIncubations(carrierID uint, page, pageSize int) ([]model.IncubationRecord, int64, error) {
	var records []model.IncubationRecord
	var total int64
	excludeSub := "AND NOT EXISTS (SELECT 1 FROM major_changes WHERE enterprise_id = incubation_records.enterprise_id AND change_type = '入孵协议文件' AND status = 'pending')"
	q := r.db.Model(&model.IncubationRecord{}).Where("carrier_id = ? AND status = 'pending' "+excludeSub, carrierID)
	q.Count(&total)
	err := q.Preload("Enterprise").Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
}

func (r *CarrierRepo) FindIncubationByID(id uint) (*model.IncubationRecord, error) {
	var record model.IncubationRecord
	err := r.db.Preload("Enterprise").First(&record, id).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *CarrierRepo) UpdateIncubationStatus(id uint, status string) error {
	return r.db.Model(&model.IncubationRecord{}).Where("id = ?", id).Update("status", status).Error
}

func (r *CarrierRepo) ListPendingChanges(carrierID uint, page, pageSize int) ([]model.MajorChange, int64, error) {
	var changes []model.MajorChange
	var total int64
	q := r.db.Model(&model.MajorChange{}).
		Joins("JOIN incubation_records ON incubation_records.enterprise_id = major_changes.enterprise_id").
		Where("incubation_records.carrier_id = ? AND major_changes.status = 'pending'", carrierID)
	q.Count(&total)
	err := q.Preload("Enterprise").Order("major_changes.created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&changes).Error
	return changes, total, err
}

func (r *CarrierRepo) FindChangeByID(id uint) (*model.MajorChange, error) {
	var change model.MajorChange
	err := r.db.First(&change, id).Error
	if err != nil {
		return nil, err
	}
	return &change, nil
}

func (r *CarrierRepo) UpdateChangeStatus(id uint, status string) error {
	return r.db.Model(&model.MajorChange{}).Where("id = ?", id).Update("status", status).Error
}

func (r *CarrierRepo) ListEnterpriseApplicationsForCarrier(carrierID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	var apps []model.PolicyApplication
	var total int64
	q := r.db.Model(&model.PolicyApplication{}).
		Joins("JOIN incubation_records ON incubation_records.enterprise_id = policy_applications.applicant_id").
		Where("incubation_records.carrier_id = ? AND policy_applications.status = 'pending' AND policy_applications.applicant_type = 'enterprise'", carrierID)
	q.Count(&total)
	err := q.Preload("Policy").Order("policy_applications.created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}

func (r *CarrierRepo) FindPolicyApplicationByID(id uint) (*model.PolicyApplication, error) {
	var app model.PolicyApplication
	err := r.db.Preload("Policy").First(&app, id).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *CarrierRepo) UpdateApplicationStatus(id uint, status string) error {
	return r.db.Model(&model.PolicyApplication{}).Where("id = ?", id).Update("status", status).Error
}

func (r *CarrierRepo) ListActiveCampaigns(page, pageSize int) ([]model.PerformanceCampaign, int64, error) {
	var campaigns []model.PerformanceCampaign
	var total int64
	q := r.db.Model(&model.PerformanceCampaign{}).Where("is_active = true")
	q.Count(&total)
	err := q.Preload("Template").Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&campaigns).Error
	return campaigns, total, err
}

func (r *CarrierRepo) CreatePerformanceSubmission(sub *model.PerformanceSubmission) error {
	return r.db.Create(sub).Error
}

func (r *CarrierRepo) FindUserIDByCarrierID(carrierID uint) (uint, error) {
	var c model.Carrier
	err := r.db.Select("user_id").First(&c, carrierID).Error
	return c.UserID, err
}

func (r *CarrierRepo) FindGovernmentUserIDs() ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.User{}).Where("role = ?", "government").Pluck("id", &ids).Error
	return ids, err
}
