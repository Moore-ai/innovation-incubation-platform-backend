package service

import (
	"time"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"
	"innovation-incubation-platform-backend/internal/repository"
	"gorm.io/gorm"
)

type GovernmentService struct {
	repo *repository.GovernmentRepo
	db   *gorm.DB
	sm   *statemachine.StateMachine
}

func NewGovernmentService(repo *repository.GovernmentRepo, db *gorm.DB) *GovernmentService {
	return &GovernmentService{repo: repo, db: db, sm: statemachine.DefaultApprovalSM()}
}

func (s *GovernmentService) CreatePolicyTemplate(req *dto.PolicyTemplateReq) (*model.PolicyTemplate, error) {
	t := &model.PolicyTemplate{
		Name: req.Name, Description: req.Description,
		FormSchema: req.FormSchema, TargetRole: req.TargetRole,
	}
	if err := s.repo.CreatePolicyTemplate(t); err != nil {
		return nil, errcode.ErrInternal
	}
	return t, nil
}

func (s *GovernmentService) PublishPolicy(req *dto.PublishPolicyReq) (*model.Policy, error) {
	now := time.Now()
	p := &model.Policy{
		TemplateID:    req.TemplateID,
		Title:         req.Title,
		Conditions:    req.Conditions,
		SubsidyAmount: req.SubsidyAmount,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Status:        "published",
		PublishedAt:   &now,
	}
	if err := s.repo.CreatePolicy(p); err != nil {
		return nil, errcode.ErrInternal
	}
	return p, nil
}

func (s *GovernmentService) ListPolicies(page, pageSize int) ([]model.Policy, int64, error) {
	return s.repo.ListPolicies(page, pageSize)
}

func (s *GovernmentService) SearchEnterprises(keyword string, page, pageSize int) ([]model.Enterprise, int64, error) {
	return s.repo.SearchEnterprises(keyword, page, pageSize)
}

func (s *GovernmentService) GetEnterprise(id uint) (*model.Enterprise, error) {
	return s.repo.FindEnterpriseByID(id)
}

func (s *GovernmentService) EditEnterprise(id uint, req *dto.EnterpriseEditReq) (*model.Enterprise, error) {
	ent, err := s.repo.FindEnterpriseByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	ent.Name = req.Name
	ent.Industry = req.Industry
	ent.Scale = req.Scale
	ent.Address = req.Address
	ent.LegalPerson = req.LegalPerson
	ent.ContactName = req.ContactName
	ent.ContactPhone = req.ContactPhone
	if err := s.repo.UpdateEnterprise(ent); err != nil {
		return nil, errcode.ErrInternal
	}
	return ent, nil
}

func (s *GovernmentService) SearchCarriers(keyword string, page, pageSize int) ([]model.Carrier, int64, error) {
	return s.repo.SearchCarriers(keyword, page, pageSize)
}

func (s *GovernmentService) ReviewPolicyApplication(appID uint, req *dto.ReviewReq) error {
	newStatus, err := s.sm.Transition("pending", req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.repo.UpdateApplicationStatus(appID, newStatus)
	s.db.Create(&model.Approval{
		TargetType: "policy",
		TargetID:   appID,
		Step:       "gov_review",
		Action:     req.Action,
		Comment:    req.Comment,
	})
	return nil
}

func (s *GovernmentService) ListPolicyApplications(page, pageSize int) ([]model.PolicyApplication, int64, error) {
	return s.repo.ListPolicyApplicationsForReview(page, pageSize)
}

func (s *GovernmentService) CreatePerformanceTemplate(req *dto.PerformanceTemplateReq) (*model.PerformanceTemplate, error) {
	t := &model.PerformanceTemplate{Name: req.Name, Year: req.Year, FormSchema: req.FormSchema}
	if err := s.repo.CreatePerformanceTemplate(t); err != nil {
		return nil, errcode.ErrInternal
	}
	return t, nil
}

func (s *GovernmentService) StartPerformanceCampaign(req *dto.PerformanceCampaignReq) (*model.PerformanceCampaign, error) {
	c := &model.PerformanceCampaign{
		TemplateID: req.TemplateID,
		Name:       req.Name,
		Year:       req.Year,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
		IsActive:   true,
	}
	if err := s.repo.CreatePerformanceCampaign(c); err != nil {
		return nil, errcode.ErrInternal
	}
	return c, nil
}

func (s *GovernmentService) ListPerformanceSubmissions(page, pageSize int) ([]model.PerformanceSubmission, int64, error) {
	return s.repo.ListPerformanceSubmissions(page, pageSize)
}

func (s *GovernmentService) ScoreSubmission(subID uint, req *dto.ScoreReq) error {
	_, err := s.repo.FindPerformanceSubmission(subID)
	if err != nil {
		return errcode.ErrNotFound
	}
	s.repo.UpdateSubmissionScore(subID, req.Status, req.Score)
	s.db.Create(&model.Approval{
		TargetType: "performance",
		TargetID:   subID,
		Step:       "gov_review",
		Action:     req.Status,
		Comment:    req.Comment,
	})
	return nil
}
