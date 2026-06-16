package service

import (
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"
	"innovation-incubation-platform-backend/internal/repository"
	"gorm.io/gorm"
)

type EnterpriseService struct {
	repo       *repository.EnterpriseRepo
	commonRepo *repository.CommonRepo
	db         *gorm.DB
	sm         *statemachine.StateMachine
}

func NewEnterpriseService(repo *repository.EnterpriseRepo, commonRepo *repository.CommonRepo, db *gorm.DB) *EnterpriseService {
	return &EnterpriseService{repo: repo, commonRepo: commonRepo, db: db, sm: statemachine.DefaultApprovalSM()}
}

func (s *EnterpriseService) ApplyIncubation(userID uint, req *dto.IncubationApplyReq) (*model.IncubationRecord, error) {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	record := &model.IncubationRecord{
		EnterpriseID:    ent.ID,
		CarrierID:       req.CarrierID,
		IncubateStatus:  "in_incubation",
		IncubateStart:   req.IncubateStart,
		IncubateEnd:     req.IncubateEnd,
		AgreementFileID: req.AgreementFileID,
		Status:          "pending",
	}
	if err := s.repo.CreateIncubation(record); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: "incubation",
		TargetID:   record.ID,
		Step:       "carrier_review",
		Action:     "submit",
		ReviewerID: 0,
	})
	return record, nil
}

func (s *EnterpriseService) GetIncubation(id uint) (*model.IncubationRecord, error) {
	record, err := s.repo.FindIncubationByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return record, nil
}

func (s *EnterpriseService) ListMyIncubation(userID uint, page, pageSize int) ([]model.IncubationRecord, int64, error) {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, 0, errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	return s.repo.ListIncubationByEnterprise(ent.ID, page, pageSize)
}

func (s *EnterpriseService) ApplyChange(userID uint, req *dto.ChangeApplyReq) (*model.MajorChange, error) {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	change := &model.MajorChange{
		EnterpriseID:  ent.ID,
		ChangeType:    req.ChangeType,
		ChangeContent: req.ChangeContent,
		OldValue:      nil,
		NewValue:      req.NewValue,
		Status:        "pending",
	}
	if err := s.repo.CreateChange(change); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: "major_change",
		TargetID:   change.ID,
		Step:       "carrier_review",
		Action:     "submit",
	})
	return change, nil
}

func (s *EnterpriseService) GetChange(id uint) (*model.MajorChange, error) {
	change, err := s.repo.FindChangeByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return change, nil
}

func (s *EnterpriseService) ListMyChanges(userID uint, page, pageSize int) ([]model.MajorChange, int64, error) {
	ent, _ := s.repo.FindEnterpriseByUserID(userID)
	return s.repo.ListChangesByEnterprise(ent.ID, page, pageSize)
}

func (s *EnterpriseService) ReeditChange(id uint, userID uint, req *dto.ChangeApplyReq) (*model.MajorChange, error) {
	change, err := s.repo.FindChangeByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	if change.Status != "returned" {
		return nil, errcode.ErrStatusInvalid.WithMsg("只有被退回的变更才能重新编辑")
	}
	change.ChangeType = req.ChangeType
	change.ChangeContent = req.ChangeContent
	change.NewValue = req.NewValue
	change.Status = "pending"
	if err := s.repo.UpdateChange(change); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: "major_change",
		TargetID:   change.ID,
		Step:       "carrier_review",
		Action:     "submit",
	})
	return change, nil
}

func (s *EnterpriseService) ListAvailablePolicies(role string, page, pageSize int) ([]model.Policy, int64, error) {
	return s.commonRepo.ListPoliciesByTarget(role, page, pageSize)
}

func (s *EnterpriseService) ApplyPolicy(userID uint, policyID uint, req *dto.PolicyApplyReq) (*model.PolicyApplication, error) {
	ent, _ := s.repo.FindEnterpriseByUserID(userID)
	policy, err := s.commonRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}
	if policy.Status != "published" {
		return nil, errcode.ErrStatusInvalid.WithMsg("该政策当前不可申报")
	}
	app := &model.PolicyApplication{
		PolicyID:      policyID,
		ApplicantID:   ent.ID,
		ApplicantType: "enterprise",
		FormData:      req.FormData,
		Status:        "pending",
	}
	if err := s.commonRepo.CreatePolicyApplication(app); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: "policy",
		TargetID:   app.ID,
		Step:       "carrier_review",
		Action:     "submit",
	})
	return app, nil
}

func (s *EnterpriseService) ListMyApplications(userID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	ent, _ := s.repo.FindEnterpriseByUserID(userID)
	return s.commonRepo.ListApplicationsByApplicant("enterprise", ent.ID, page, pageSize)
}
