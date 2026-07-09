package repository

import (
	"gorm.io/gorm"
	"innovation-incubation-platform-backend/internal/model"
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
		Where("status = ? AND (target_role = ? OR target_role = 'both')", model.PolicyPublished, role)
	q.Count(&total)
	err := q.Order("created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&policies).Error
	return policies, total, err
}

func (r *CommonRepo) FindPolicyByID(id uint) (*model.Policy, error) {
	var policy model.Policy
	err := r.db.First(&policy, id).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *CommonRepo) FindUserIDsByRole(role string) ([]uint, error) {
	var ids []uint
	err := r.db.Model(&model.User{}).Where("role = ?", role).Pluck("id", &ids).Error
	return ids, err
}

func (r *CommonRepo) CreatePolicyApplication(app *model.PolicyApplication) error {
	return r.db.Create(app).Error
}

func (r *CommonRepo) HasBlockingPolicyApplication(applicantType string, applicantID uint, policyID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.PolicyApplication{}).
		Where("applicant_type = ? AND applicant_id = ? AND policy_id = ? AND status NOT IN ?",
			applicantType, applicantID, policyID, reSubmittablePolicyApplicationStatuses()).
		Count(&count).Error
	return count > 0, err
}

func (r *CommonRepo) CreatePolicyApplicationIfNotBlocked(app *model.PolicyApplication) (bool, error) {
	created := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("LOCK TABLE policy_applications IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
				return err
			}
		}

		var count int64
		if err := tx.Model(&model.PolicyApplication{}).
			Where("applicant_type = ? AND applicant_id = ? AND policy_id = ? AND status NOT IN ?",
				app.ApplicantType, app.ApplicantID, app.PolicyID, reSubmittablePolicyApplicationStatuses()).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		if err := tx.Create(app).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

func reSubmittablePolicyApplicationStatuses() []string {
	return []string{
		string(model.ApprovalRejected),
		string(model.ApprovalReturned),
	}
}

func (r *CommonRepo) ListApplicationsByApplicant(applicantType string, applicantID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	var apps []model.PolicyApplication
	var total int64
	q := r.db.Model(&model.PolicyApplication{}).
		Where("applicant_type = ? AND applicant_id = ?", applicantType, applicantID)
	q.Count(&total)
	err := q.Preload("Policy").Order("created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&apps).Error
	return apps, total, err
}
