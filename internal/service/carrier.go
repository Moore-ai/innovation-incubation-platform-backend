package service

import (
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"
	"gorm.io/gorm"
)

var validReviewActions = map[string]bool{
	string(model.ActionApprove): true,
	string(model.ActionReject):  true,
	string(model.ActionReturn):  true,
}

func validateReviewAction(action string) error {
	if !validReviewActions[action] {
		return errcode.ErrInvalidParams.WithMsg("审核操作无效，必须为 approve、reject 或 return")
	}
	return nil
}

type CarrierService struct {
	repo       *repository.CarrierRepo
	commonRepo *repository.CommonRepo
	db         *gorm.DB
	sm         *statemachine.StateMachine
}

func NewCarrierService(repo *repository.CarrierRepo, commonRepo *repository.CommonRepo, db *gorm.DB) *CarrierService {
	return &CarrierService{repo: repo, commonRepo: commonRepo, db: db, sm: statemachine.DefaultApprovalSM()}
}

func (s *CarrierService) ReviewIncubation(carrierUserID uint, incubationID uint, req *dto.ReviewReq) error {
	if err := validateReviewAction(req.Action); err != nil {
		return err
	}
	carrier, _ := s.repo.FindCarrierByUserID(carrierUserID)
	record, err := s.repo.FindIncubationByID(incubationID)
	if err != nil {
		return errcode.ErrNotFound
	}
	if record.CarrierID != carrier.ID {
		return errcode.ErrForbidden
	}
	newStatus, err := s.sm.Transition(string(record.Status), req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&model.IncubationRecord{}).Where("id = ?", incubationID).Update("status", newStatus)
		tx.Create(&model.Approval{
			TargetType: model.TargetIncubation,
			TargetID:   incubationID,
			Step:       model.StepCarrierReview,
			Action:     model.ApprovalAction(req.Action),
			Comment:    req.Comment,
			ReviewerID: carrierUserID,
		})
		return nil
	})
	return nil
}

func (s *CarrierService) ListPendingIncubations(userID uint, page, pageSize int) ([]model.IncubationRecord, int64, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	return s.repo.ListPendingIncubations(carrier.ID, page, pageSize)
}

func (s *CarrierService) ReviewChange(carrierUserID uint, changeID uint, req *dto.ReviewReq) error {
	if err := validateReviewAction(req.Action); err != nil {
		return err
	}
	_, _ = s.repo.FindCarrierByUserID(carrierUserID)
	change, err := s.repo.FindChangeByID(changeID)
	if err != nil {
		return errcode.ErrNotFound
	}
	newStatus, err := s.sm.Transition(string(change.Status), req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&model.MajorChange{}).Where("id = ?", changeID).Update("status", newStatus)
		tx.Create(&model.Approval{
			TargetType: model.TargetMajorChange,
			TargetID:   changeID,
			Step:       model.StepCarrierReview,
			Action:     model.ApprovalAction(req.Action),
			Comment:    req.Comment,
			ReviewerID: carrierUserID,
		})
		return nil
	})
	return nil
}

func (s *CarrierService) ListPendingChanges(userID uint, page, pageSize int) ([]model.MajorChange, int64, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	return s.repo.ListPendingChanges(carrier.ID, page, pageSize)
}

func (s *CarrierService) UpdateInfo(userID uint, req *dto.CarrierInfoReq) (*model.Carrier, error) {
	carrier, err := s.repo.FindCarrierByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	carrier.Name = req.Name
	carrier.Type = req.Type
	carrier.Address = req.Address
	carrier.Area = req.Area
	carrier.ManagerName = req.ManagerName
	carrier.ContactPhone = req.ContactPhone
	carrier.Description = req.Description
	if err := s.repo.UpdateCarrier(carrier); err != nil {
		return nil, errcode.ErrInternal
	}
	return carrier, nil
}

func (s *CarrierService) GetMyInfo(userID uint) (*model.Carrier, error) {
	return s.repo.FindCarrierByUserID(userID)
}

func (s *CarrierService) ListAvailableCarrierPolicies(page, pageSize int) ([]model.Policy, int64, error) {
	return s.commonRepo.ListPoliciesByTarget(string(model.RoleCarrier), page, pageSize)
}

func (s *CarrierService) ApplyCarrierPolicy(userID uint, policyID uint, req *dto.PolicyApplyReq) (*model.PolicyApplication, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	_, err := s.commonRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}
	app := &model.PolicyApplication{
		PolicyID:      policyID,
		ApplicantID:   carrier.ID,
		ApplicantType: model.ApplicantCarrier,
		FormData:      req.FormData,
		Status:        model.ApprovalPending,
	}
	s.commonRepo.CreatePolicyApplication(app)
	s.db.Create(&model.Approval{
		TargetType: model.TargetPolicy,
		TargetID:   app.ID,
		Step:       model.StepGovReview,
		Action:     model.ActionSubmit,
	})
	return app, nil
}

func (s *CarrierService) ListEnterpriseApplications(userID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	return s.repo.ListEnterpriseApplicationsForCarrier(carrier.ID, page, pageSize)
}

func (s *CarrierService) ReviewEnterprisePolicyApplication(carrierUserID uint, appID uint, req *dto.ReviewReq) error {
	if err := validateReviewAction(req.Action); err != nil {
		return err
	}
	_, _ = s.repo.FindCarrierByUserID(carrierUserID)
	app, err := s.repo.FindPolicyApplicationByID(appID)
	if err != nil {
		return errcode.ErrNotFound
	}
	newStatus, err := s.sm.Transition(string(app.Status), req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&model.PolicyApplication{}).Where("id = ?", appID).Update("status", newStatus)
		tx.Create(&model.Approval{
			TargetType: model.TargetPolicy,
			TargetID:   appID,
			Step:       model.StepGovReview,
			Action:     model.ApprovalAction(req.Action),
			Comment:    req.Comment,
			ReviewerID: carrierUserID,
		})
		return nil
	})
	return nil
}

func (s *CarrierService) ListActiveCampaigns(page, pageSize int) ([]model.PerformanceCampaign, int64, error) {
	return s.repo.ListActiveCampaigns(page, pageSize)
}

func (s *CarrierService) SubmitPerformance(userID uint, campaignID uint, req *dto.PerformanceSubmitReq) (*model.PerformanceSubmission, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	sub := &model.PerformanceSubmission{
		CampaignID: campaignID,
		CarrierID:  carrier.ID,
		FormData:   req.FormData,
		Status:     model.ApprovalPending,
	}
	if err := s.repo.CreatePerformanceSubmission(sub); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: model.TargetPerformance,
		TargetID:   sub.ID,
		Step:       model.StepGovReview,
		Action:     model.ActionSubmit,
	})
	return sub, nil
}
