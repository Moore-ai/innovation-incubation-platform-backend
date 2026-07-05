package service

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/statemachine"

	"gorm.io/gorm"
)

type EnterpriseService struct {
	repo        *repository.EnterpriseRepo
	carrierRepo *repository.CarrierRepo
	commonRepo  *repository.CommonRepo
	db          *gorm.DB
	sm          *statemachine.StateMachine
	notifSvc    *NotificationService
	assigner    *Assigner
	followRepo  *repository.PolicyFollowRepo
}

func NewEnterpriseService(repo *repository.EnterpriseRepo, carrierRepo *repository.CarrierRepo, commonRepo *repository.CommonRepo, db *gorm.DB, notifSvc *NotificationService, assigner *Assigner, followRepo *repository.PolicyFollowRepo) *EnterpriseService {
	return &EnterpriseService{repo: repo, carrierRepo: carrierRepo, commonRepo: commonRepo, db: db, sm: statemachine.DefaultApprovalSM(), notifSvc: notifSvc, assigner: assigner, followRepo: followRepo}
}

func (s *EnterpriseService) GetMyEnterpriseInfo(userID uint) (*model.Enterprise, error) {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	return ent, nil
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func validateDateRange(start, end string) error {
	if !datePattern.MatchString(start) {
		return errcode.ErrInvalidParams.WithMsg("入孵开始日期格式错误，应为 YYYY-MM-DD")
	}
	if end != "" && !datePattern.MatchString(end) {
		return errcode.ErrInvalidParams.WithMsg("入孵结束日期格式错误，应为 YYYY-MM-DD")
	}
	return nil
}

func (s *EnterpriseService) ApplyIncubation(userID uint, req *dto.IncubationApplyReq) (*model.IncubationRecord, error) {
	if req.CarrierID == 0 {
		return nil, errcode.ErrInvalidParams.WithMsg("请选择所属载体")
	}
	if err := validateDateRange(req.IncubateStart, req.IncubateEnd); err != nil {
		return nil, err
	}
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	// 校验：上一次入驻未结束，不允许再次申请
	active, actErr := s.repo.FindActiveIncubation(ent.ID)
	if actErr == nil && active != nil {
		return nil, errcode.ErrInvalidParams.WithMsg(
			fmt.Sprintf("您当前仍有未结束的入驻记录（结束时间：%s），请等待结束后再申请", active.IncubateEnd),
		)
	}
	// 若企业表尚未填写统一社会信用代码，则从入驻申请中补齐（18位）
	if ent.CreditCode == "" && req.CreditCode != "" {
		if len(req.CreditCode) != 18 {
			return nil, errcode.ErrInvalidParams.WithMsg("统一社会信用代码必须为 18 位")
		}
		if err := s.db.Model(&model.Enterprise{}).
			Where("id = ?", ent.ID).
			Updates(map[string]any{
				"credit_code": req.CreditCode,
			}).Error; err != nil {
			return nil, errcode.ErrInternal.WithMsg("写入统一社会信用代码失败")
		}
		ent.CreditCode = req.CreditCode
	}
	// 同样地，若企业表的企业名称为空，也从申请中补齐
	if ent.Name == "" && req.EnterpriseName != "" {
		if err := s.db.Model(&model.Enterprise{}).
			Where("id = ?", ent.ID).
			Update("name", req.EnterpriseName).Error; err != nil {
			return nil, errcode.ErrInternal.WithMsg("写入企业名称失败")
		}
		ent.Name = req.EnterpriseName
	}
	record := &model.IncubationRecord{
		EnterpriseID:    ent.ID,
		CarrierID:       req.CarrierID,
		IncubateStatus:  model.IncubateInIncubation,
		IncubateStart:   req.IncubateStart,
		IncubateEnd:     req.IncubateEnd,
		AgreementFileID: req.AgreementFileID,
		Status:          model.ApprovalPending,
	}
	if err := s.repo.CreateIncubation(record); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: model.TargetIncubation,
		TargetID:   record.ID,
		Step:       model.StepCarrierReview,
		Action:     model.ActionSubmit,
		ReviewerID: 0,
	})
	// 通知所属载体
	var carrierUserID uint
	s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", req.CarrierID).Take(&carrierUserID)
	if carrierUserID > 0 {
		s.notifSvc.Send(carrierUserID, model.NotifIncubationPending,
			"有一份新的入驻申请待审核",
			fmt.Sprintf("企业「%s」提交了入驻申请", ent.Name),
			model.TargetIncubation, record.ID)
	}
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

var allowedChangeTypes = []string{
	"企业名称",
	"统一社会信用代码",
	"所属行业",
	"企业规模",
	"企业地址",
	"法定代表人",
	"入孵协议文件",
}

func validateChangeType(t string) error {
	for _, v := range allowedChangeTypes {
		if v == t {
			return nil
		}
	}
	return errcode.ErrInvalidParams.WithMsg("不允许修改该指标，请选择可变更指标：" + strings.Join(allowedChangeTypes, "、"))
}

func ListChangeTypes() []string {
	r := make([]string, len(allowedChangeTypes))
	copy(r, allowedChangeTypes)
	return r
}

// ListDicts 返回入驻申请表单所需的全部字典数据
func ListDicts() dto.DictResponse {
	makeItems := func(pairs ...string) []dto.DictItem {
		items := make([]dto.DictItem, 0, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			items = append(items, dto.DictItem{Value: pairs[i], Label: pairs[i+1]})
		}
		return items
	}

	return dto.DictResponse{
		EnterpriseNatures: makeItems(
			"国有企业", "国有企业",
			"集体企业", "集体企业",
			"有限责任公司", "有限责任公司",
			"股份有限公司", "股份有限公司",
			"私营企业", "私营企业",
			"港澳台投资企业", "港澳台投资企业",
			"外商投资企业", "外商投资企业",
			"其他", "其他",
		),
		FoundingCategories: makeItems(
			"科研人员创业", "科研人员创业",
			"大学生创业", "大学生创业",
			"海归人员创业", "海归人员创业",
			"企业衍生", "企业衍生",
			"引进转化", "引进转化",
			"其他", "其他",
		),
		IndustryCategories: makeItems(
			"农、林、牧、渔业", "农、林、牧、渔业",
			"采矿业", "采矿业",
			"制造业", "制造业",
			"电力、热力、燃气及水生产和供应业", "电力、热力、燃气及水生产和供应业",
			"建筑业", "建筑业",
			"批发和零售业", "批发和零售业",
			"交通运输、仓储和邮政业", "交通运输、仓储和邮政业",
			"住宿和餐饮业", "住宿和餐饮业",
			"信息传输、软件和信息技术服务业", "信息传输、软件和信息技术服务业",
			"金融业", "金融业",
			"房地产业", "房地产业",
			"租赁和商务服务业", "租赁和商务服务业",
			"科学研究和技术服务业", "科学研究和技术服务业",
			"水利、环境和公共设施管理业", "水利、环境和公共设施管理业",
			"居民服务、修理和其他服务业", "居民服务、修理和其他服务业",
			"教育", "教育",
			"卫生和社会工作", "卫生和社会工作",
			"文化、体育和娱乐业", "文化、体育和娱乐业",
			"公共管理、社会保障和社会组织", "公共管理、社会保障和社会组织",
			"国际组织", "国际组织",
		),
		HighTechFields: makeItems(
			"电子信息技术", "电子信息技术",
			"软件和信息技术服务业", "软件和信息技术服务业",
			"生物与新医药技术", "生物与新医药技术",
			"航空航天技术", "航空航天技术",
			"新材料技术", "新材料技术",
			"高技术服务业", "高技术服务业",
			"新能源及节能技术", "新能源及节能技术",
			"资源与环境技术", "资源与环境技术",
			"高新技术改造传统产业", "高新技术改造传统产业",
			"其他", "其他",
		),
		IncubateStatuses: makeItems(
			"在孵", "在孵企业",
			"毕业", "毕业企业",
			"退出", "退出企业",
		),
	}
}

func (s *EnterpriseService) ApplyChange(userID uint, req *dto.ChangeApplyReq) (*model.MajorChange, error) {
	if err := validateChangeType(req.ChangeType); err != nil {
		return nil, err
	}
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
		Status:        model.ApprovalPending,
	}
	if err := s.repo.CreateChange(change); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: model.TargetMajorChange,
		TargetID:   change.ID,
		Step:       model.StepCarrierReview,
		Action:     model.ActionSubmit,
	})
	// 通知企业所属载体（通过入驻记录查找最新关联的载体）
	var carrierID uint
	s.db.Model(&model.IncubationRecord{}).Select("carrier_id").Where("enterprise_id = ?", ent.ID).Order("created_at DESC").Limit(1).Take(&carrierID)
	if carrierID > 0 {
		var carrierUserID uint
		s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", carrierID).Take(&carrierUserID)
		if carrierUserID > 0 {
			s.notifSvc.Send(carrierUserID, model.NotifChangePending,
				"有一条新的变更申请待审核",
				fmt.Sprintf("企业「%s」提交了「%s」变更", ent.Name, req.ChangeType),
				model.TargetMajorChange, change.ID)
		}
	}
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
	if err := validateChangeType(req.ChangeType); err != nil {
		return nil, err
	}
	change, err := s.repo.FindChangeByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	if change.Status != model.ApprovalReturned {
		return nil, errcode.ErrStatusInvalid.WithMsg("只有被退回的变更才能重新编辑")
	}
	change.ChangeType = req.ChangeType
	change.ChangeContent = req.ChangeContent
	change.NewValue = req.NewValue
	change.Status = model.ApprovalPending
	if err := s.repo.UpdateChange(change); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: model.TargetMajorChange,
		TargetID:   change.ID,
		Step:       model.StepCarrierReview,
		Action:     model.ActionSubmit,
	})
	// 通知载体
	var entCarrierID uint
	s.db.Model(&model.IncubationRecord{}).Select("carrier_id").Where("enterprise_id = ?", change.EnterpriseID).Order("created_at DESC").Limit(1).Take(&entCarrierID)
	if entCarrierID > 0 {
		var carrierUserID uint
		s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", entCarrierID).Take(&carrierUserID)
		if carrierUserID > 0 {
			s.notifSvc.Send(carrierUserID, model.NotifChangePending,
				"有一条新的变更申请待审核",
				fmt.Sprintf("企业重新提交了「%s」变更", change.ChangeType),
				model.TargetMajorChange, change.ID)
		}
	}
	return change, nil
}

func (s *EnterpriseService) ApplyDeletion(userID uint, reason string) error {
	if reason == "" {
		return errcode.ErrInvalidParams.WithMsg("请填写注销原因")
	}
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	var existing int64
	s.db.Model(&model.AccountDeletionRequest{}).Where("user_id = ? AND status = ?", userID, model.ApprovalPending).Count(&existing)
	if existing > 0 {
		return errcode.ErrStatusInvalid.WithMsg("您已有一笔待处理的注销申请，请等待审核结果")
	}
	req := &model.AccountDeletionRequest{
		UserID:       userID,
		Role:         string(model.UserRoleEnterprise),
		EnterpriseID: &ent.ID,
		Reason:       reason,
		Status:       model.ApprovalPending,
	}
	if err := s.db.Create(req).Error; err != nil {
		return errcode.ErrInternal
	}
	// 通知政务（轮询分配）
	uid, err := s.assigner.Next("government")
	if err != nil {
		slog.Error("assigner next failed", "error", err)
	} else {
		s.notifSvc.Send(uid, model.NotifDeletionApplied,
			"有一条新的注销申请待审核",
			fmt.Sprintf("企业「%s」提交了账号注销申请", ent.Name),
			model.TargetAccountDeletion, req.ID)
	}
	return nil
}

func (s *EnterpriseService) ListAvailablePolicies(userID uint, role string, page, pageSize int) ([]model.Policy, int64, error) {
	policies, total, err := s.commonRepo.ListPoliciesByTarget(role, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return policies, total, nil
}

func (s *EnterpriseService) ApplyPolicy(userID uint, policyID uint, req *dto.PolicyApplyReq) (*model.PolicyApplication, error) {
	ent, _ := s.repo.FindEnterpriseByUserID(userID)
	policy, err := s.commonRepo.FindPolicyByID(policyID)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("政策不存在")
	}
	if policy.Status != model.PolicyPublished {
		return nil, errcode.ErrStatusInvalid.WithMsg("该政策当前不可申报")
	}
	app := &model.PolicyApplication{
		PolicyID:      policyID,
		ApplicantID:   ent.ID,
		ApplicantType: model.ApplicantEnterprise,
		Materials:     req.Materials,
		Status:        model.ApprovalPending,
	}
	if err := s.commonRepo.CreatePolicyApplication(app); err != nil {
		return nil, errcode.ErrInternal
	}
	s.db.Create(&model.Approval{
		TargetType: model.TargetPolicy,
		TargetID:   app.ID,
		Step:       model.StepCarrierReview,
		Action:     model.ActionSubmit,
	})
	// 通知所属载体
	var carrierID uint
	s.db.Model(&model.IncubationRecord{}).Select("carrier_id").Where("enterprise_id = ?", ent.ID).Order("created_at DESC").Limit(1).Take(&carrierID)
	if carrierID > 0 {
		var carrierUserID uint
		s.db.Model(&model.Carrier{}).Select("user_id").Where("id = ?", carrierID).Take(&carrierUserID)
		if carrierUserID > 0 {
			s.notifSvc.Send(carrierUserID, model.NotifApplicationPending,
				"有一条新的政策申报待审核",
				fmt.Sprintf("企业「%s」申报了政策「%s」", ent.Name, policy.Title),
				model.TargetPolicy, app.ID)
		}
	}
	return app, nil
}

func (s *EnterpriseService) ListMyApplications(userID uint, page, pageSize int) ([]model.PolicyApplication, int64, error) {
	ent, _ := s.repo.FindEnterpriseByUserID(userID)
	return s.commonRepo.ListApplicationsByApplicant(string(model.ApplicantEnterprise), ent.ID, page, pageSize)
}

func (s *EnterpriseService) FollowPolicy(userID, policyID uint) error {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	exists, err := s.followRepo.Exists(ent.ID, policyID)
	if err != nil {
		return errcode.ErrInternal
	}
	if exists {
		return errcode.ErrDuplicate.WithMsg("已关注该政策")
	}
	if _, err := s.commonRepo.FindPolicyByID(policyID); err != nil {
		return errcode.ErrNotFound.WithMsg("政策不存在")
	}
	if err := s.followRepo.Create(ent.ID, policyID); err != nil {
		return errcode.ErrDuplicate.WithMsg("已关注该政策")
	}
	return nil
}

func (s *EnterpriseService) UnfollowPolicy(userID, policyID uint) error {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	if err := s.followRepo.Delete(ent.ID, policyID); err != nil {
		return errcode.ErrNotFound.WithMsg("未关注该政策")
	}
	return nil
}

func (s *EnterpriseService) ListFollowedPolicies(userID uint, page, pageSize int) ([]model.PolicyFollow, int64, error) {
	ent, err := s.repo.FindEnterpriseByUserID(userID)
	if err != nil {
		return nil, 0, errcode.ErrNotFound.WithMsg("企业信息未找到")
	}
	return s.followRepo.ListByEnterprise(ent.ID, page, pageSize)
}

func (s *EnterpriseService) ListCarriers(page, pageSize int) ([]model.Carrier, int64, error) {
	return s.carrierRepo.ListAll(page, pageSize)
}

func (s *EnterpriseService) GetCarrier(id uint) (*model.Carrier, error) {
	c, err := s.carrierRepo.FindByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound.WithMsg("载体不存在")
	}
	return c, nil
}
