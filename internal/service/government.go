package service

import (
	"time"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/pkg/errcode"
	"innovation-incubation-platform-backend/internal/pkg/statemachine"
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

type PolicyTemplateReq struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	FormSchema  model.JSONMap `json:"form_schema"`
	TargetRole  string        `json:"target_role"`
}

func (s *GovernmentService) CreatePolicyTemplate(req *PolicyTemplateReq) (*model.PolicyTemplate, error) {
	t := &model.PolicyTemplate{
		Name: req.Name, Description: req.Description,
		FormSchema: req.FormSchema, TargetRole: req.TargetRole,
	}
	if err := s.repo.CreatePolicyTemplate(t); err != nil {
		return nil, errcode.ErrInternal
	}
	return t, nil
}

type PublishPolicyReq struct {
	TemplateID    uint          `json:"template_id"`
	Title         string        `json:"title"`
	Conditions    model.JSONMap `json:"conditions"`
	SubsidyAmount string        `json:"subsidy_amount"`
	StartDate     string        `json:"start_date"`
	EndDate       string        `json:"end_date"`
}

func (s *GovernmentService) PublishPolicy(req *PublishPolicyReq) (*model.Policy, error) {
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

type EnterpriseEditReq struct {
	Name         string `json:"name"`
	Industry     string `json:"industry"`
	Scale        string `json:"scale"`
	Address      string `json:"address"`
	LegalPerson  string `json:"legal_person"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
}

func (s *GovernmentService) EditEnterprise(id uint, req *EnterpriseEditReq) (*model.Enterprise, error) {
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

func (s *GovernmentService) ReviewPolicyApplication(appID uint, req *ReviewReq) error {
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

type PerformanceTemplateReq struct {
	Name       string        `json:"name"`
	Year       int           `json:"year"`
	FormSchema model.JSONMap `json:"form_schema"`
}

func (s *GovernmentService) CreatePerformanceTemplate(req *PerformanceTemplateReq) (*model.PerformanceTemplate, error) {
	t := &model.PerformanceTemplate{Name: req.Name, Year: req.Year, FormSchema: req.FormSchema}
	if err := s.repo.CreatePerformanceTemplate(t); err != nil {
		return nil, errcode.ErrInternal
	}
	return t, nil
}

type PerformanceCampaignReq struct {
	TemplateID uint   `json:"template_id"`
	Name       string `json:"name"`
	Year       int    `json:"year"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
}

func (s *GovernmentService) StartPerformanceCampaign(req *PerformanceCampaignReq) (*model.PerformanceCampaign, error) {
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

type ScoreReq struct {
	Score   float64 `json:"score"`
	Status  string  `json:"status"` // approved, rejected
	Comment string  `json:"comment"`
}

func (s *GovernmentService) ScoreSubmission(subID uint, req *ScoreReq) error {
	sub, err := s.repo.FindPerformanceSubmission(subID)
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
	_ = sub
	return nil
}
