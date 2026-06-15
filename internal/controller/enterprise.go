package controller

import (
	"strconv"

	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/pkg/errcode"
	"innovation-incubation-platform-backend/internal/pkg/response"
	"innovation-incubation-platform-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type EnterpriseController struct {
	svc *service.EnterpriseService
}

func NewEnterpriseController(svc *service.EnterpriseService) *EnterpriseController {
	return &EnterpriseController{svc: svc}
}

func (ctl *EnterpriseController) ApplyIncubation(c *gin.Context) {
	var req service.IncubationApplyReq
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
	var req service.ChangeApplyReq
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
	var req service.ChangeApplyReq
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
	policies, total, err := ctl.svc.ListAvailablePolicies("enterprise", page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, policies, total, page, pageSize)
}

func (ctl *EnterpriseController) ApplyPolicy(c *gin.Context) {
	policyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req service.PolicyApplyReq
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

func (ctl *EnterpriseController) RecommendPolicy(c *gin.Context) {
	// TODO: implement after AI service is ready
	response.Success(c, gin.H{"message": "AI recommend endpoint"})
}

func (ctl *EnterpriseController) PrefillApplication(c *gin.Context) {
	// TODO: implement after AI service is ready
	response.Success(c, gin.H{"message": "AI prefill endpoint"})
}
