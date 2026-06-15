package controller

import (
	"strconv"

	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"
	"innovation-incubation-platform-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type GovernmentController struct {
	svc *service.GovernmentService
}

func NewGovernmentController(svc *service.GovernmentService) *GovernmentController {
	return &GovernmentController{svc: svc}
}

func (ctl *GovernmentController) CreatePolicyTemplate(c *gin.Context) {
	var req service.PolicyTemplateReq
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
	var req service.PublishPolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	p, err := ctl.svc.PublishPolicy(&req)
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
	var req service.EnterpriseEditReq
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
	var req service.ReviewReq
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
	var req service.PerformanceTemplateReq
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
	var req service.PerformanceCampaignReq
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
	var req service.ScoreReq
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
