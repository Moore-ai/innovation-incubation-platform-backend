package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"
	"innovation-incubation-platform-backend/internal/repository"
	"gorm.io/gorm"
)

type GovernmentService struct {
	repo     *repository.GovernmentRepo
	db       *gorm.DB
	sm       *statemachine.StateMachine
	aiSvc    *AIService
	notifSvc *NotificationService
}

func NewGovernmentService(repo *repository.GovernmentRepo, db *gorm.DB, aiSvc *AIService, notifSvc *NotificationService) *GovernmentService {
	return &GovernmentService{repo: repo, db: db, sm: statemachine.DefaultApprovalSM(), aiSvc: aiSvc, notifSvc: notifSvc}
}

func (s *GovernmentService) CreatePolicyTemplate(req *dto.PolicyTemplateReq) (*model.PolicyTemplate, error) {
	t := &model.PolicyTemplate{
		Name: req.Name, Description: req.Description,
		FormSchema: req.FormSchema, TargetRole: model.TargetRole(req.TargetRole),
	}
	if err := s.repo.CreatePolicyTemplate(t); err != nil {
		return nil, errcode.ErrInternal
	}
	return t, nil
}

func (s *GovernmentService) PublishPolicy(ctx context.Context, req *dto.PublishPolicyReq) (*model.Policy, error) {
	now := time.Now()
	p := &model.Policy{
		TemplateID:    req.TemplateID,
		Title:         req.Title,
		Conditions:    req.Conditions,
		SubsidyAmount: req.SubsidyAmount,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Status:        model.PolicyPublished,
		PublishedAt:   &now,
	}
	if err := s.repo.CreatePolicy(p); err != nil {
		return nil, errcode.ErrInternal
	}

	// 同步 AI 提取 — 失败则删除 policy 并返回错误
	if err := s.aiSvc.ExtractPolicy(ctx, p); err != nil {
		slog.Error("AI extract policy failed, rolling back", "policy_id", p.ID, "error", err)
		if delErr := s.repo.DeletePolicy(p.ID); delErr != nil {
			slog.Error("failed to rollback policy after AI extract failure", "policy_id", p.ID, "delete_error", delErr)
		}
		return nil, errcode.ErrAIService.WithMsg("AI提取政策字段失败，请重试")
	}

	// 通知目标用户群
	var targetRole string
	s.db.Model(&model.PolicyTemplate{}).Select("target_role").Where("id = ?", req.TemplateID).Take(&targetRole)
	if targetRole != "" {
		if targetRole == "both" || targetRole == "enterprise" {
			entIDs, _ := s.repo.FindUserIDsByRole("enterprise")
			for _, uid := range entIDs {
				s.notifSvc.Send(uid, model.NotifPolicyPublished,
					"有一项新政策可供申报",
					fmt.Sprintf("新政策「%s」已发布", p.Title),
					model.TargetPolicy, p.ID)
			}
		}
		if targetRole == "both" || targetRole == "carrier" {
			carrierIDs, _ := s.repo.FindUserIDsByRole("carrier")
			for _, uid := range carrierIDs {
				s.notifSvc.Send(uid, model.NotifPolicyPublished,
					"有一项新政策可供申报",
					fmt.Sprintf("新政策「%s」已发布", p.Title),
					model.TargetPolicy, p.ID)
			}
		}
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
	if err := validateReviewAction(req.Action); err != nil {
		return err
	}
	newStatus, err := s.sm.Transition(string(model.ApprovalPending), req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}
	s.repo.UpdateApplicationStatus(appID, newStatus)
	s.db.Create(&model.Approval{
		TargetType: model.TargetPolicy,
		TargetID:   appID,
		Step:       model.StepGovReview,
		Action:     model.ApprovalAction(req.Action),
		Comment:    req.Comment,
	})

	// 通知申请人
	var applicant model.PolicyApplication
	if err := s.db.First(&applicant, appID).Error; err == nil {
		if applicant.ApplicantType == model.ApplicantEnterprise {
			var entUserID uint
			s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", applicant.ApplicantID).Take(&entUserID)
			if entUserID > 0 {
				actionMsg := map[string]string{string(model.ActionApprove): "通过", string(model.ActionReject): "拒绝", string(model.ActionReturn): "退回"}[req.Action]
				s.notifSvc.Send(entUserID, model.NotifApplicationReviewed,
					fmt.Sprintf("政策申报已被%s", actionMsg),
					fmt.Sprintf("您的政策申报已被政务%s", actionMsg),
					model.TargetPolicy, appID)
			}
		}
	}
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
	if req.Status != string(model.ActionApprove) && req.Status != string(model.ActionReject) {
		return errcode.ErrInvalidParams.WithMsg("评分状态必须为 approve 或 reject")
	}
	sub, err := s.repo.FindPerformanceSubmission(subID)
	if err != nil {
		return errcode.ErrNotFound
	}
	s.repo.UpdateSubmissionScore(subID, req.Status, req.Score)
	s.db.Create(&model.Approval{
		TargetType: model.TargetPerformance,
		TargetID:   subID,
		Step:       model.StepGovReview,
		Action:     model.ApprovalAction(req.Status),
		Comment:    req.Comment,
	})

	// 通知载体
	var carrierUserID uint
	s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", sub.CarrierID).Take(&carrierUserID)
	if carrierUserID > 0 {
		s.notifSvc.Send(carrierUserID, model.NotifPerformanceScored,
			"绩效考核已被评分",
			"您的绩效考核已被政务评分",
			model.TargetPerformance, subID)
	}
	return nil
}
