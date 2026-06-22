package service

import (
	"time"
	"fmt"

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
	notifSvc   *NotificationService
}

func NewCarrierService(repo *repository.CarrierRepo, commonRepo *repository.CommonRepo, db *gorm.DB, notifSvc *NotificationService) *CarrierService {
	return &CarrierService{repo: repo, commonRepo: commonRepo, db: db, sm: statemachine.DefaultApprovalSM(), notifSvc: notifSvc}
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
	// 协议文件检查
	if record.AgreementFileID == nil {
		return errcode.ErrInvalidParams.WithMsg("审核失败，该企业尚未上传入孵协议文件")
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

	// 通知企业
	var entUserID uint
	s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", record.EnterpriseID).Take(&entUserID)
	if entUserID > 0 {
		actionMsg := map[string]string{
			string(model.ActionApprove): "通过",
			string(model.ActionReject):  "拒绝",
			string(model.ActionReturn):  "退回",
		}[req.Action]
		s.notifSvc.Send(entUserID, model.NotifIncubationReviewed,
			fmt.Sprintf("入驻申请已被%s", actionMsg),
			fmt.Sprintf("您的入驻申请已被载体%s", actionMsg),
			model.TargetIncubation, incubationID)
	}

	return nil
}

func (s *CarrierService) ListPendingIncubations(userID uint, page, pageSize int) ([]model.IncubationRecord, int64, error) {
	carrier, _ := s.repo.FindCarrierByUserID(userID)
	return s.repo.ListPendingIncubations(carrier.ID, page, pageSize)
}

func (s *CarrierService) CompleteIncubation(carrierUserID uint, incubationID uint) error {
	carrier, err := s.repo.FindCarrierByUserID(carrierUserID)
	if err != nil {
		return errcode.ErrForbidden
	}
	record, err := s.repo.FindIncubationByID(incubationID)
	if err != nil {
		return errcode.ErrNotFound
	}
	if record.CarrierID != carrier.ID {
		return errcode.ErrForbidden
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
			Step:       model.StepCarrierReview,
			Action:     model.ActionApprove,
			ReviewerID: carrierUserID,
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
		return nil
	})
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
		if req.Action == string(model.ActionApprove) {
			ent := &model.Enterprise{}
			if err := tx.First(ent, change.EnterpriseID).Error; err != nil {
				return err
			}
			applyChange(ent, change, tx)
			if err := tx.Save(ent).Error; err != nil {
				return err
			}
		}
		return nil
	})
	// 通知企业
	var entUserID uint
	s.db.Model(&model.Enterprise{}).Select("user_id").Where("id = ?", change.EnterpriseID).Take(&entUserID)
	if entUserID > 0 {
		if change.ChangeType == "入孵协议文件" && req.Action == string(model.ActionApprove) {
			s.notifSvc.Send(entUserID, model.NotifChangeReviewed,
				"协议文件变更已被批准",
				"您的协议文件变更已被批准，请重新提交入驻申请并上传新协议",
				model.TargetMajorChange, changeID)
		} else {
			actionMsg := map[string]string{
				string(model.ActionApprove): "通过",
				string(model.ActionReject):  "拒绝",
				string(model.ActionReturn):  "退回",
			}[req.Action]
			s.notifSvc.Send(entUserID, model.NotifChangeReviewed,
				fmt.Sprintf("变更申请已被%s", actionMsg),
				fmt.Sprintf("您的「%s」变更申请已被载体%s", change.ChangeType, actionMsg),
				model.TargetMajorChange, changeID)
		}
	}

	return nil
}

// applyChange maps ChangeType to Enterprise struct fields and applies the new value.
func applyChange(ent *model.Enterprise, change *model.MajorChange, db *gorm.DB) {
	v, ok := change.NewValue[change.ChangeType].(string)
	if !ok {
		return
	}
	switch change.ChangeType {
	case "企业名称":
		ent.Name = v
	case "统一社会信用代码":
		ent.CreditCode = v
	case "所属行业":
		ent.Industry = v
	case "企业规模":
		ent.Scale = v
	case "企业地址":
		ent.Address = v
	case "法定代表人":
		ent.LegalPerson = v
	case "入孵协议文件":
		recordID, _ := change.NewValue["incubation_record_id"].(float64)
		if recordID == 0 {
			return
		}
		var record model.IncubationRecord
		if err := db.First(&record, uint(recordID)).Error; err != nil {
			return
		}
		if record.AgreementFileID != nil {
			db.Delete(&model.File{}, *record.AgreementFileID)
		}
		db.Delete(&record)
	}
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

	if req.Action == string(model.ActionApprove) {
		govIDs, _ := s.repo.FindGovernmentUserIDs()
		for _, uid := range govIDs {
			s.notifSvc.Send(uid, model.NotifApplicationCarrierApproved,
				"有一条政策申报已通过载体审核",
				"有一条政策申报已通过载体审核，请尽快处理",
				model.TargetPolicy, appID)
		}
	}

	return nil
}

func (s *CarrierService) ApplyDeletion(userID uint, reason string) error {
	if reason == "" {
		return errcode.ErrInvalidParams.WithMsg("请填写注销原因")
	}
	carrier, err := s.repo.FindCarrierByUserID(userID)
	if err != nil {
		return errcode.ErrNotFound.WithMsg("载体信息未找到")
	}
	req := &model.AccountDeletionRequest{
		UserID:    userID,
		Role:      string(model.RoleCarrier),
		CarrierID: &carrier.ID,
		Reason:    reason,
		Status:    model.ApprovalPending,
	}
	if err := s.db.Create(req).Error; err != nil {
		return errcode.ErrInternal
	}
	// 通知政务
	var govIDs []uint
	s.db.Model(&model.User{}).Select("id").Where("role = ?", "government").Pluck("id", &govIDs)
	for _, uid := range govIDs {
		s.notifSvc.Send(uid, model.NotifDeletionApplied,
			"有一条新的注销申请待审核",
			fmt.Sprintf("载体「%s」提交了账号注销申请", carrier.Name),
			model.TargetAccountDeletion, req.ID)
	}
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

	// 通知政务
	govIDs, _ := s.repo.FindGovernmentUserIDs()
	for _, uid := range govIDs {
		s.notifSvc.Send(uid, model.NotifPerformanceSubmitted,
			"有一条新的绩效考核申报待评分",
			"有一条新的绩效考核申报待评分",
			model.TargetPerformance, sub.ID)
	}

	return sub, nil
}
