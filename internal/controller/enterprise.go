package controller

import (
	"strconv"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type EnterpriseController struct {
	svc   *service.EnterpriseService
	aiSvc *service.AIService
}

func NewEnterpriseController(svc *service.EnterpriseService, aiSvc *service.AIService) *EnterpriseController {
	return &EnterpriseController{svc: svc, aiSvc: aiSvc}
}

func (ctl *EnterpriseController) ApplyIncubation(c *gin.Context) {
	var req dto.IncubationApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	record, err := ctl.svc.ApplyIncubation(middleware.GetUserID(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, record)
}

func (ctl *EnterpriseController) GetIncubation(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	record, err := ctl.svc.GetIncubation(uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, record)
}

func (ctl *EnterpriseController) ListMyIncubation(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	records, total, err := ctl.svc.ListMyIncubation(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, records, total, page, pageSize)
}

func (ctl *EnterpriseController) ApplyChange(c *gin.Context) {
	var req dto.ChangeApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	change, err := ctl.svc.ApplyChange(middleware.GetUserID(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, change)
}

func (ctl *EnterpriseController) GetChange(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	change, err := ctl.svc.GetChange(uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, change)
}

func (ctl *EnterpriseController) ListMyChanges(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	changes, total, err := ctl.svc.ListMyChanges(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, changes, total, page, pageSize)
}

func (ctl *EnterpriseController) ReeditChange(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ChangeApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	change, err := ctl.svc.ReeditChange(uint(id), middleware.GetUserID(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, change)
}

func (ctl *EnterpriseController) ListPolicies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	policies, total, err := ctl.svc.ListAvailablePolicies(middleware.GetUserID(c), string(model.RoleEnterprise), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, policies, total, page, pageSize)
}

func (ctl *EnterpriseController) ApplyPolicy(c *gin.Context) {
	policyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.PolicyApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	app, err := ctl.svc.ApplyPolicy(middleware.GetUserID(c), uint(policyID), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, app)
}

func (ctl *EnterpriseController) ListMyApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	apps, total, err := ctl.svc.ListMyApplications(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, apps, total, page, pageSize)
}

func (ctl *EnterpriseController) GetMyEnterpriseInfo(c *gin.Context) {
	ent, err := ctl.svc.GetMyEnterpriseInfo(middleware.GetUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, ent)
}

func (ctl *EnterpriseController) ApplyDeletion(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("请填写注销原因"))
		return
	}
	if err := ctl.svc.ApplyDeletion(middleware.GetUserID(c), req.Reason); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *EnterpriseController) ListChangeTypes(c *gin.Context) {
	response.Success(c, service.ListChangeTypes())
}

func (ctl *EnterpriseController) RecommendPolicy(c *gin.Context) {
	policyID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("政策ID参数无效"))
		return
	}
	result, rErr := ctl.aiSvc.MatchPolicy(c.Request.Context(), middleware.GetUserID(c), uint(policyID))
	if rErr != nil {
		response.Error(c, rErr)
		return
	}
	response.Success(c, result)
}

func (ctl *EnterpriseController) PrefillApplication(c *gin.Context) {
	var req dto.PrefillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if req.PolicyID == 0 {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("policy_id 不能为空"))
		return
	}
	data, rErr := ctl.aiSvc.PrefillApplication(c.Request.Context(), middleware.GetUserID(c), req.PolicyID)
	if rErr != nil {
		response.Error(c, rErr)
		return
	}
	response.Success(c, data)
}
