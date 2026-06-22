package controller

import (
	"strconv"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"
	"innovation-incubation-platform-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type CarrierController struct {
	svc *service.CarrierService
}

func NewCarrierController(svc *service.CarrierService) *CarrierController {
	return &CarrierController{svc: svc}
}

func (ctl *CarrierController) ReviewIncubation(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if err := ctl.svc.ReviewIncubation(middleware.GetUserID(c), uint(id), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *CarrierController) CompleteIncubation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}
	if err := ctl.svc.CompleteIncubation(middleware.GetUserID(c), uint(id)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *CarrierController) ListPendingIncubations(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	records, total, err := ctl.svc.ListPendingIncubations(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, records, total, page, pageSize)
}

func (ctl *CarrierController) ReviewChange(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if err := ctl.svc.ReviewChange(middleware.GetUserID(c), uint(id), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *CarrierController) ListPendingChanges(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	changes, total, err := ctl.svc.ListPendingChanges(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, changes, total, page, pageSize)
}

func (ctl *CarrierController) UpdateInfo(c *gin.Context) {
	var req dto.CarrierInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	carrier, err := ctl.svc.UpdateInfo(middleware.GetUserID(c), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, carrier)
}

func (ctl *CarrierController) GetMyInfo(c *gin.Context) {
	carrier, err := ctl.svc.GetMyInfo(middleware.GetUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, carrier)
}

func (ctl *CarrierController) ListPolicies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	policies, total, err := ctl.svc.ListAvailableCarrierPolicies(page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, policies, total, page, pageSize)
}

func (ctl *CarrierController) ApplyPolicy(c *gin.Context) {
	policyID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.PolicyApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	app, err := ctl.svc.ApplyCarrierPolicy(middleware.GetUserID(c), uint(policyID), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, app)
}

func (ctl *CarrierController) ListEnterpriseApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	apps, total, err := ctl.svc.ListEnterpriseApplications(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, apps, total, page, pageSize)
}

func (ctl *CarrierController) ReviewEnterpriseApplication(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if err := ctl.svc.ReviewEnterprisePolicyApplication(middleware.GetUserID(c), uint(id), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *CarrierController) ListCampaigns(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	campaigns, total, err := ctl.svc.ListActiveCampaigns(page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, campaigns, total, page, pageSize)
}

func (ctl *CarrierController) SubmitPerformance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.PerformanceSubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	sub, err := ctl.svc.SubmitPerformance(middleware.GetUserID(c), uint(id), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, sub)
}
