package service

import (
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"
	"innovation-incubation-platform-backend/internal/repository"
	"gorm.io/gorm"
)

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
	carrier, _ := s.repo.FindCarrierByUserID(carrierUserID)
	record, err := s.repo.FindIncubationByID(incubationID)
	if err != nil {
		return errcode.ErrNotFound
	}
	if record.CarrierID != carrier.ID {
		return errcode.ErrForbidden
	}
	newStatus, err := s.sm.Transition(record.Status, req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&model.IncubationRecord{}).Where("id = ?", incubationID).Update("status", newStatus)
		tx.Create(&model.Approval{
			TargetType: "incubation",
			TargetID:   incubationID,
			Step:       "carrier_review",
			Action:     req.Action,
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
	_, _ = s.repo.FindCarrierByUserID(carrierUserID)
	change, err := s.repo.FindChangeByID(changeID)
	if err != nil {
		return errcode.ErrNotFound
	}
	newStatus, err := s.sm.Transition(change.Status, req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&model.MajorChange{}).Where("id = ?", changeID).Update("status", newStatus)
		tx.Create(&model.Approval{
			TargetType: "major_change",
			TargetID:   changeID,
			Step:       "carrier_review",
			Action:     req.Action,
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
	return s.commonRepo.ListPoliciesByTarget("carrier", page, pageSize)
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
		ApplicantType: "carrier",
		FormData:      req.FormData,
		Status:        "pending",
	}
	s.commonRepo.CreatePolicyApplication(app)
	s.db.Create(&model.Approval{
		TargetType: "policy",
		TargetID:   app.ID,
		Step:       "gov_review",
		Action:     "submit",
	})
	return app, nil
}

func (s *CarrierService) ListEnterpriseApplications(userID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	return s.repo.ListEnterpriseApplicationsForCarrier(carrier.ID, page, pageSize)
}

func (s *CarrierService) ReviewEnterprisePolicyApplication(carrierUserID uint, appID uint, req *dto.ReviewReq) error {
	_, _ = s.repo.FindCarrierByUserID(carrierUserID)
	app, err := s.repo.FindPolicyApplicationByID(appID)
	if err != nil {
		return errcode.ErrNotFound
	}
	newStatus, err := s.sm.Transition(app.Status, req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.db.Transaction(func(tx *gorm.DB) error {
		tx.Model(&model.PolicyApplication{}).Where("id = ?", appID).Update("status", newStatus)
		tx.Create(&model.Approval{
			TargetType: "policy",
			TargetID:   appID,
			Step:       "gov_review",
			Action:     req.Action,
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
		Status:     "pending",
	}
	if err := s.repo.CreatePerformanceSubmission(sub); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: "performance",
		TargetID:   sub.ID,
		Step:       "gov_review",
		Action:     "submit",
	})
	return sub, nil
}
