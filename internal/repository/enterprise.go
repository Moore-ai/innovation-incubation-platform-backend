package repository

import (
	"time"

	"innovation-incubation-platform-backend/internal/model"
	"gorm.io/gorm"
)

type EnterpriseRepo struct {
	db *gorm.DB
}

func NewEnterpriseRepo(db *gorm.DB) *EnterpriseRepo {
	return &EnterpriseRepo{db: db}
}

func (r *EnterpriseRepo) CreateIncubation(record *model.IncubationRecord) error {
	return r.db.Create(record).Error
}

func (r *EnterpriseRepo) FindIncubationByID(id uint) (*model.IncubationRecord, error) {
	var record model.IncubationRecord
	err := r.db.Preload("Enterprise").Preload("Carrier").First(&record, id).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *EnterpriseRepo) ListIncubationByEnterprise(enterpriseID uint, page, pageSize int) ([]model.IncubationRecord, int64, error) {
	var records []model.IncubationRecord
	var total int64
	q := r.db.Model(&model.IncubationRecord{}).Where("enterprise_id = ?", enterpriseID)
	q.Count(&total)
	err := q.Preload("Carrier").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error
	return records, total, err
}

// FindActiveIncubation 查找该企业当前仍处于在孵/未结束状态的入驻记录
// "未结束"定义为：状态为已通过且孵化状态为在孵，且入驻结束日期晚于今天
func (r *EnterpriseRepo) FindActiveIncubation(enterpriseID uint) (*model.IncubationRecord, error) {
	var record model.IncubationRecord
	today := time.Now().Format("2006-01-02")
	err := r.db.Where(
		"enterprise_id = ? AND status = ? AND incubate_status = ? AND incubate_end >= ?",
		enterpriseID, model.ApprovalApproved, model.IncubateInIncubation, today,
	).Order("incubate_end DESC").First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *EnterpriseRepo) FindEnterpriseByUserID(userID uint) (*model.Enterprise, error) {
	var ent model.Enterprise
	err := r.db.Where("user_id = ?", userID).First(&ent).Error
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

func (r *EnterpriseRepo) CreateChange(change *model.MajorChange) error {
	return r.db.Create(change).Error
}

func (r *EnterpriseRepo) FindChangeByID(id uint) (*model.MajorChange, error) {
	var change model.MajorChange
	err := r.db.First(&change, id).Error
	if err != nil {
		return nil, err
	}
	return &change, nil
}

func (r *EnterpriseRepo) ListChangesByEnterprise(entID uint, page, pageSize int) ([]model.MajorChange, int64, error) {
	var changes []model.MajorChange
	var total int64
	q := r.db.Model(&model.MajorChange{}).Where("enterprise_id = ?", entID)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page-1)*pageSize).Limit(pageSize).Find(&changes).Error
	return changes, total, err
}

func (r *EnterpriseRepo) UpdateChange(change *model.MajorChange) error {
	return r.db.Save(change).Error
}

func (r *EnterpriseRepo) FindUserIDByEnterpriseID(entID uint) (uint, error) {
	var ent model.Enterprise
	err := r.db.Select("user_id").First(&ent, entID).Error
	return ent.UserID, err
}

func (r *EnterpriseRepo) FindApprovedApplicationsPaginated(entID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	var apps []model.PolicyApplication
	var total int64
	q := r.db.Model(&model.PolicyApplication{}).
		Where("applicant_id = ? AND applicant_type = ? AND status = ?", entID, string(model.ApplicantEnterprise), string(model.ApprovalApproved))
	q.Count(&total)
	err := q.Preload("Policy").Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}

func (r *EnterpriseRepo) FindApprovedApplications(entID uint) ([]model.PolicyApplication, error) {
	var apps []model.PolicyApplication
	err := r.db.Where("applicant_type = ? AND applicant_id = ? AND status IN ?", string(model.ApplicantEnterprise), entID, []string{string(model.ApprovalApproved)}).
		Preload("Policy").Order("created_at DESC").Find(&apps).Error
	return apps, err
}
