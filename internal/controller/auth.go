package controller

import (
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/response"
	"innovation-incubation-platform-backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	svc *service.AuthService
}

func NewAuthController(svc *service.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

func (ctl *AuthController) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	resp, err := ctl.svc.Register(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (ctl *AuthController) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errcode.ErrInvalidParams.WithMsg(err.Error()))
		return
	}
	resp, err := ctl.svc.Login(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

func (ctl *AuthController) GetMe(c *gin.Context) {
	userID := middleware.GetUserID(c)
	info, err := ctl.svc.GetMe(userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, info)
}
