package controller

import (
	"strconv"

	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type GovernmentController struct {
	svc *service.GovernmentService
}

func NewGovernmentController(svc *service.GovernmentService) *GovernmentController {
	return &GovernmentController{svc: svc}
}

func (ctl *GovernmentController) CreatePolicyTemplate(c *gin.Context) {
	var req dto.PolicyTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	t, err := ctl.svc.CreatePolicyTemplate(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, t)
}

func (ctl *GovernmentController) PublishPolicy(c *gin.Context) {
	var req dto.PublishPolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	p, err := ctl.svc.PublishPolicy(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, p)
}

func (ctl *GovernmentController) ListPolicies(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	policies, total, err := ctl.svc.ListPolicies(page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, policies, total, page, pageSize)
}

func (ctl *GovernmentController) SearchEnterprises(c *gin.Context) {
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ents, total, err := ctl.svc.SearchEnterprises(keyword, page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, ents, total, page, pageSize)
}

func (ctl *GovernmentController) GetEnterprise(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	ent, err := ctl.svc.GetEnterprise(uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, ent)
}

func (ctl *GovernmentController) EditEnterprise(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.EnterpriseEditReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	ent, err := ctl.svc.EditEnterprise(uint(id), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, ent)
}

func (ctl *GovernmentController) SearchCarriers(c *gin.Context) {
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	carriers, total, err := ctl.svc.SearchCarriers(keyword, page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, carriers, total, page, pageSize)
}

func (ctl *GovernmentController) ReviewPolicyApplication(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if err := ctl.svc.ReviewPolicyApplication(uint(id), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *GovernmentController) ListPolicyApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	apps, total, err := ctl.svc.ListPolicyApplications(page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, apps, total, page, pageSize)
}

func (ctl *GovernmentController) CreatePerformanceTemplate(c *gin.Context) {
	var req dto.PerformanceTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	t, err := ctl.svc.CreatePerformanceTemplate(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, t)
}

func (ctl *GovernmentController) StartCampaign(c *gin.Context) {
	var req dto.PerformanceCampaignReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	campaign, err := ctl.svc.StartPerformanceCampaign(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, campaign)
}

func (ctl *GovernmentController) ListSubmissions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	subs, total, err := ctl.svc.ListPerformanceSubmissions(page, pageSize)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, subs, total, page, pageSize)
}

func (ctl *GovernmentController) ScoreSubmission(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req dto.ScoreReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	if err := ctl.svc.ScoreSubmission(uint(id), &req); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *GovernmentController) CompleteIncubation(c *gin.Context) {
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

func (ctl *GovernmentController) DeleteEnterprise(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}
	if err := ctl.svc.DeleteEnterprise(uint(id), middleware.GetUserID(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *GovernmentController) DeleteCarrier(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}
	if err := ctl.svc.DeleteCarrier(uint(id), middleware.GetUserID(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}

func (ctl *GovernmentController) ListDeletionRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	list, total, err := ctl.svc.ListDeletionRequests(page, pageSize, status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.SuccessPage(c, list, total, page, pageSize)
}

func (ctl *GovernmentController) ReviewDeletionRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, errcode.ErrInvalidParams)
		return
	}
	var req struct {
		Action  string `json:"action"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Action != "approve" && req.Action != "reject") {
		response.Error(c, errcode.ErrInvalidParams.WithMsg("action 必须为 approve 或 reject"))
		return
	}
	if err := ctl.svc.ReviewDeletionRequest(middleware.GetUserID(c), uint(id), req.Action, req.Comment); err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, nil)
}
