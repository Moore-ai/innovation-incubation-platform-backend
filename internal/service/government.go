package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"

	"gorm.io/gorm"
)

type GovernmentService struct {
	repo         *repository.GovernmentRepo
	deletionRepo *repository.DeletionRepo
	followRepo   *repository.PolicyFollowRepo
	db           *gorm.DB
	sm           *statemachine.StateMachine
	policySM     *statemachine.StateMachine
	aiSvc        *AIService
	notifSvc     *NotificationService
}

func NewGovernmentService(repo *repository.GovernmentRepo, deletionRepo *repository.DeletionRepo, followRepo *repository.PolicyFollowRepo, db *gorm.DB, aiSvc *AIService, notifSvc *NotificationService) *GovernmentService {
	return &GovernmentService{repo: repo, deletionRepo: deletionRepo, followRepo: followRepo, db: db, sm: statemachine.DefaultApprovalSM(), policySM: statemachine.PolicyApprovalSM(), aiSvc: aiSvc, notifSvc: notifSvc}
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
		TemplateID:   req.TemplateID,
		Title:        req.Title,
		Requirements: req.Requirements,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Status:       model.PolicyPublished,
		PublishedAt:  &now,
	}
	// 验证申报材料必要性
	if req.Requirements != nil {
		for _, m := range req.Requirements.ApplicationMaterials {
			if m.Necessity != model.NecessityRequired && m.Necessity != model.NecessityNotRequired {
				return nil, errcode.ErrInvalidParams.WithMsg(fmt.Sprintf("材料必要性无效: %s, 必须为「necessary」或「unnecessary」", m.Necessity))
			}
		}
	}
	// 验证法律依据文件存在
	if req.Requirements != nil {
		for _, basis := range req.Requirements.LegalBasis {
			var f model.File
			if err := s.db.First(&f, basis.FileID).Error; err != nil {
				return nil, errcode.ErrInvalidParams.WithMsg(fmt.Sprintf("法律依据文件不存在: file_id=%d", basis.FileID))
			}
		}
	}

	if err := s.aiSvc.ExtractPolicy(ctx, p); err != nil {
		return nil, errcode.ErrAIService.WithMsg("AI提取政策字段失败，请重试")
	}
	if err := s.repo.CreatePolicy(p); err != nil {
		return nil, errcode.ErrInternal
	}

	// 通知目标用户群
	var targetRole string
	s.db.Model(&model.PolicyTemplate{}).Select("target_role").Where("id = ?", req.TemplateID).Take(&targetRole)
	if targetRole != "" {
		if targetRole == string(model.RoleBoth) || targetRole == string(model.RoleEnterprise) {
			entIDs, _ := s.repo.FindUserIDsByRole(string(model.RoleEnterprise))
			for _, uid := range entIDs {
				if err := s.notifSvc.Send(uid, model.NotifPolicyPublished,
					"有一项新政策可供申报",
					fmt.Sprintf("新政策「%s」已发布", p.Title),
					model.TargetPolicy, p.ID); err != nil {
					slog.Error("notification failed", "error", err)
				}
			}
		}
		if targetRole == string(model.RoleBoth) || targetRole == string(model.RoleCarrier) {
			carrierIDs, _ := s.repo.FindUserIDsByRole(string(model.RoleCarrier))
			for _, uid := range carrierIDs {
				if err := s.notifSvc.Send(uid, model.NotifPolicyPublished,
					"有一项新政策可供申报",
					fmt.Sprintf("新政策「%s」已发布", p.Title),
					model.TargetPolicy, p.ID); err != nil {
					slog.Error("notification failed", "error", err)
				}
			}
		}
	}

	return p, nil
}

func (s *GovernmentService) ListPolicies(page, pageSize int) ([]model.Policy, int64, error) {
	return s.repo.ListPolicies(page, pageSize)
}

func (s *GovernmentService) UpdatePolicy(ctx context.Context, policyID uint, req *dto.PublishPolicyReq) error {
	p, err := s.repo.FindPolicyByID(policyID)
	if err != nil {
		return errcode.ErrNotFound
	}
	p.Title = req.Title
	p.Requirements = req.Requirements
	p.StartDate = req.StartDate
	p.EndDate = req.EndDate
	// 验证申报材料必要性
	if req.Requirements != nil {
		for _, m := range req.Requirements.ApplicationMaterials {
			if m.Necessity != model.NecessityRequired && m.Necessity != model.NecessityNotRequired {
				return errcode.ErrInvalidParams.WithMsg(fmt.Sprintf("材料必要性无效: %s，必须为「必要」或「非必要」", m.Necessity))
			}
		}
	}
	// 验证法律依据文件存在
	if req.Requirements != nil {
		for _, basis := range req.Requirements.LegalBasis {
			var f model.File
			if err := s.db.First(&f, basis.FileID).Error; err != nil {
				return errcode.ErrInvalidParams.WithMsg(fmt.Sprintf("法律依据文件不存在: file_id=%d", basis.FileID))
			}
		}
	}
	// Re-extract AI fields after policy update
	if err := s.aiSvc.ExtractPolicy(ctx, p); err != nil {
		slog.Error("AI extract policy failed after update", "policy_id", policyID, "error", err)
		// Don't fail the update — the policy fields are saved even if AI extraction fails
	}
	if err := s.repo.UpdatePolicy(p); err != nil {
		return errcode.ErrInternal
	}
	// 通知关注者
	entIDs, _ := s.followRepo.FindEnterpriseIDsByPolicy(policyID)
	for _, entID := range entIDs {
		var userID uint
		s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", entID).First(&userID)
		if userID > 0 {
			if err := s.notifSvc.Send(userID, model.NotifPolicyUpdated,
				"您关注的政策已更新",
				fmt.Sprintf("您关注的政策「%s」已更新", p.Title),
				model.TargetPolicy, policyID); err != nil {
				slog.Error("policy update notification failed", "policy_id", policyID, "user_id", userID, "error", err)
			}
		}
	}
	return nil
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

func (s *GovernmentService) ReviewPolicyApplication(govUserID uint, appID uint, req *dto.ReviewReq) error {
	if err := validateReviewAction(req.Action); err != nil {
		return err
	}

	var app model.PolicyApplication
	if err := s.db.First(&app, appID).Error; err != nil {
		return errcode.ErrNotFound.WithMsg("申报记录不存在")
	}

	var sm *statemachine.StateMachine
	switch {
	case app.Status == model.ApprovalPending && app.ApplicantType == model.ApplicantCarrier:
		sm = s.sm
	case app.Status == model.ApprovalGovReview:
		sm = s.policySM
	case app.Status == model.ApprovalPending && app.ApplicantType == model.ApplicantEnterprise:
		return errcode.ErrStatusInvalid.WithMsg("企业申报需先由载体审核")
	default:
		return errcode.ErrStatusInvalid.WithMsg("该申请当前状态不可审核")
	}
	newStatus, err := sm.Transition(string(app.Status), req.Action)
	if err != nil {
		return errcode.ErrStatusInvalid.WithMsg(err.Error())
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.PolicyApplication{}).Where("id = ? AND status = ?", appID, app.Status).Update("status", newStatus)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errcode.ErrStatusInvalid.WithMsg("申请状态已变更，请刷新后重试")
		}
		if err := tx.Create(&model.Approval{
			TargetType: model.TargetPolicy,
			TargetID:   appID,
			Step:       model.StepGovReview,
			Action:     model.ApprovalAction(req.Action),
			Comment:    req.Comment,
			ReviewerID: govUserID,
		}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// 通知申请人
	actionMsg := map[string]string{string(model.ActionApprove): "通过", string(model.ActionReject): "拒绝", string(model.ActionReturn): "退回"}[req.Action]
	switch app.ApplicantType {
	case model.ApplicantEnterprise:
		var entUserID uint
		s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", app.ApplicantID).Take(&entUserID)
		if entUserID > 0 {
			if err := s.notifSvc.Send(entUserID, model.NotifApplicationReviewed,
				fmt.Sprintf("政策申报已被%s", actionMsg),
				fmt.Sprintf("您的政策申报已被政务%s", actionMsg),
				model.TargetPolicy, appID); err != nil {
				slog.Error("notification failed", "error", err)
			}
		}
	case model.ApplicantCarrier:
		var carrierUserID uint
		s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", app.ApplicantID).Take(&carrierUserID)
		if carrierUserID > 0 {
			if err := s.notifSvc.Send(carrierUserID, model.NotifApplicationReviewed,
				fmt.Sprintf("政策申报已被%s", actionMsg),
				fmt.Sprintf("您的政策申报已被政务%s", actionMsg),
				model.TargetPolicy, appID); err != nil {
				slog.Error("notification failed", "error", err)
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
		if err := s.notifSvc.Send(carrierUserID, model.NotifPerformanceScored,
			"绩效考核已被评分",
			"您的绩效考核已被政务评分",
			model.TargetPerformance, subID); err != nil {
			slog.Error("notification failed", "error", err)
		}
	}
	return nil
}

func (s *GovernmentService) CompleteIncubation(userID, incubationID uint) error {
	var record model.IncubationRecord
	if err := s.db.First(&record, incubationID).Error; err != nil {
		return errcode.ErrNotFound
	}
	if record.IncubateStatus != model.IncubateInIncubation {
		return errcode.ErrStatusInvalid.WithMsg("该入驻记录当前状态不可标记为孵化完成")
	}
	if record.Status != model.ApprovalApproved {
		return errcode.ErrStatusInvalid.WithMsg("入驻申请尚未通过审核，无法标记为孵化完成")
	}
	now := time.Now().Format("2006-01-02")
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.IncubationRecord{}).
			Where("id = ? AND incubate_status = ?", incubationID, model.IncubateInIncubation).
			Updates(map[string]any{
				"incubate_status": model.IncubateGraduated,
				"incubate_end":    now,
			})
		if res.Error != nil {
			return errcode.ErrInternal
		}
		if res.RowsAffected == 0 {
			return errcode.ErrStatusInvalid.WithMsg("该入驻记录当前状态不可标记为孵化完成")
		}
		tx.Create(&model.Approval{
			TargetType: model.TargetIncubation,
			TargetID:   incubationID,
			Step:       model.StepGovReview,
			Action:     model.ActionApprove,
			ReviewerID: userID,
		})
		// 通知企业
		var entUserID uint
		tx.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", record.EnterpriseID).Take(&entUserID)
		if entUserID > 0 {
			if err := s.notifSvc.Send(entUserID, model.NotifIncubationGraduated,
				"孵化已完成",
				"贵企业已完成孵化，恭喜！",
				model.TargetIncubation, incubationID); err != nil {
				return err
			}
		}
		// 通知载体
		var carrierUserID uint
		tx.Model(&model.Carrier{}).Select("user_id").Where("id = ?", record.CarrierID).Take(&carrierUserID)
		if carrierUserID > 0 {
			if err := s.notifSvc.Send(carrierUserID, model.NotifIncubationGraduated,
				"孵化已完成",
				"企业已完成孵化",
				model.TargetIncubation, incubationID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GovernmentService) ListDeletionRequests(page, pageSize int, status string) ([]model.AccountDeletionRequest, int64, error) {
	if status == "" || status == "pending" {
		return s.deletionRepo.ListPending(page, pageSize)
	}
	var list []model.AccountDeletionRequest
	var total int64
	q := s.db.Model(&model.AccountDeletionRequest{}).Where("status = ?", status)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (s *GovernmentService) ReviewDeletionRequest(govUserID uint, reqID uint, action, comment string) error {
	var r model.AccountDeletionRequest
	if err := s.db.First(&r, reqID).Error; err != nil {
		return errcode.ErrNotFound
	}
	if r.Status != model.ApprovalPending {
		return errcode.ErrStatusInvalid.WithMsg("该申请已被处理")
	}

	switch action {
	case "approve":
		// 删除前捕获企业/载体名称（用于通知）
		var entName string
		var carrierName string
		if r.EnterpriseID != nil {
			s.db.Model(&model.Enterprise{}).Select("name").Where("id = ?", *r.EnterpriseID).Take(&entName)
		}
		if r.CarrierID != nil {
			s.db.Model(&model.Carrier{}).Select("name").Where("id = ?", *r.CarrierID).Take(&carrierName)
		}

		// 单事务：通知 → 更新状态 → 执行删除
		return s.db.Transaction(func(tx *gorm.DB) error {
			// 先发送通知（用户删除前）
			if err := s.notifSvc.Send(r.UserID, model.NotifDeletionApproved,
				"账号注销申请已通过",
				"您的账号注销申请已通过，账号将被注销",
				model.TargetAccountDeletion, r.ID); err != nil {
				slog.Error("notification failed", "error", err)
			}

			// 更新申请状态
			if err := tx.Model(&r).Updates(map[string]any{
				"status":         model.ApprovalApproved,
				"reviewer_id":    govUserID,
				"review_comment": comment,
			}).Error; err != nil {
				return err
			}

			// 执行删除
			return s.executeDeletionTx(tx, &r)
		})
	case "reject":
		return s.db.Transaction(func(tx *gorm.DB) error {
			if err := s.notifSvc.Send(r.UserID, model.NotifDeletionRejected,
				"账号注销申请被拒绝",
				"您的账号注销申请已被拒绝："+comment,
				model.TargetAccountDeletion, r.ID); err != nil {
				slog.Error("notification failed", "error", err)
			}
			return tx.Model(&r).Updates(map[string]any{
				"status":         model.ApprovalRejected,
				"reviewer_id":    govUserID,
				"review_comment": comment,
			}).Error
		})
	}
	return errcode.ErrInvalidParams.WithMsg("操作必须为 approve 或 reject")
}

func (s *GovernmentService) DeleteEnterprise(entID, govUserID uint) error {
	ent, err := s.repo.FindEnterpriseByID(entID)
	if err != nil {
		return errcode.ErrNotFound
	}
	var userID uint
	s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", entID).First(&userID)
	var carrierIDs []uint
	s.db.Model(&model.IncubationRecord{}).Select("carrier_id").Where("enterprise_id = ?", entID).Pluck("carrier_id", &carrierIDs)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if userID > 0 {
			if err := tx.Delete(&model.User{}, userID).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&model.Enterprise{}, entID).Error; err != nil {
			return err
		}
		if err := tx.Where("enterprise_id = ?", entID).Delete(&model.IncubationRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("enterprise_id = ?", entID).Delete(&model.MajorChange{}).Error; err != nil {
			return err
		}
		if err := tx.Where("applicant_id = ? AND applicant_type = ?", entID, model.ApplicantEnterprise).Delete(&model.PolicyApplication{}).Error; err != nil {
			return err
		}
		if err := tx.Where("enterprise_id = ?", entID).Delete(&model.AccountDeletionRequest{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errcode.ErrInternal
	}
	for _, carrierID := range carrierIDs {
		var carrierUserID uint
		s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", carrierID).First(&carrierUserID)
		if carrierUserID > 0 {
			if err := s.notifSvc.Send(carrierUserID, model.NotifAccountDeleted,
				"企业已被注销",
				fmt.Sprintf("企业「%s」已被注销", ent.Name),
				model.TargetAccountDeletion, entID); err != nil {
				slog.Error("notification failed", "error", err)
			}
		}
	}
	return nil
}

func (s *GovernmentService) DeleteCarrier(carrierID, govUserID uint) error {
	carrier, err := s.repo.FindCarrierByID(carrierID)
	if err != nil {
		return errcode.ErrNotFound
	}
	var userID uint
	s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", carrierID).First(&userID)
	var enterpriseIDs []uint
	s.db.Model(&model.IncubationRecord{}).Select("enterprise_id").Where("carrier_id = ?", carrierID).Pluck("enterprise_id", &enterpriseIDs)

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if userID > 0 {
			if err := tx.Delete(&model.User{}, userID).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&model.Carrier{}, carrierID).Error; err != nil {
			return err
		}
		if err := tx.Where("carrier_id = ?", carrierID).Delete(&model.IncubationRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("applicant_id = ? AND applicant_type = ?", carrierID, model.ApplicantCarrier).Delete(&model.PolicyApplication{}).Error; err != nil {
			return err
		}
		if err := tx.Where("carrier_id = ?", carrierID).Delete(&model.AccountDeletionRequest{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errcode.ErrInternal
	}
	for _, entID := range enterpriseIDs {
		var entUserID uint
		s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", entID).First(&entUserID)
		if entUserID > 0 {
			if err := s.notifSvc.Send(entUserID, model.NotifAccountDeleted,
				"载体已被注销",
				fmt.Sprintf("载体「%s」已被注销", carrier.Name),
				model.TargetAccountDeletion, carrierID); err != nil {
				slog.Error("notification failed", "error", err)
			}
		}
	}
	return nil
}

// executeDeletionTx 在已有事务中执行数据删除，由 ReviewDeletionRequest 的事务调用。
func (s *GovernmentService) executeDeletionTx(tx *gorm.DB, r *model.AccountDeletionRequest) error {
	if err := tx.Delete(&model.User{}, r.UserID).Error; err != nil {
		return err
	}
	if r.EnterpriseID != nil {
		if err := tx.Delete(&model.Enterprise{}, *r.EnterpriseID).Error; err != nil {
			return err
		}
		if err := tx.Where("enterprise_id = ?", *r.EnterpriseID).Delete(&model.IncubationRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("enterprise_id = ?", *r.EnterpriseID).Delete(&model.MajorChange{}).Error; err != nil {
			return err
		}
		if err := tx.Where("applicant_id = ? AND applicant_type = ?", *r.EnterpriseID, model.ApplicantEnterprise).Delete(&model.PolicyApplication{}).Error; err != nil {
			return err
		}
	}
	if r.CarrierID != nil {
		if err := tx.Delete(&model.Carrier{}, *r.CarrierID).Error; err != nil {
			return err
		}
		if err := tx.Where("carrier_id = ?", *r.CarrierID).Delete(&model.IncubationRecord{}).Error; err != nil {
			return err
		}
	}
	// 清理关联的注销申请记录
	if err := tx.Where("user_id = ?", r.UserID).Delete(&model.AccountDeletionRequest{}).Error; err != nil {
		return err
	}
	return nil
}
