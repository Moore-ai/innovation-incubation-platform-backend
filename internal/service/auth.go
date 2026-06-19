package service

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/pkg/errcode"
	"log/slog"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repo *repository.AuthRepo
	cfg  config.JWTConfig
}

func NewAuthService(repo *repository.AuthRepo, cfg config.JWTConfig) *AuthService {
	return &AuthService{repo: repo, cfg: cfg}
}

func (s *AuthService) Register(req *dto.RegisterRequest) (*dto.LoginResponse, error) {
	if req.Phone == "" {
		return nil, errcode.ErrInvalidParams.WithMsg("手机号不能为空")
	}
	if req.Role != "enterprise" && req.Role != "carrier" {
		return nil, errcode.ErrInvalidParams.WithMsg("角色无效")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errcode.ErrInternal
	}

	user := &model.User{
		PasswordHash: string(hash),
		Role:         req.Role,
		Phone:        req.Phone,
		Email:        req.Email,
	}
	err = s.repo.CreateUser(user)
	if err != nil {
		return nil, errcode.ErrDuplicate.WithMsg("手机号已注册")
	}
	switch req.Role {
	case "enterprise":
		ent := &model.Enterprise{
			UserID:     user.ID,
			Name:       req.EnterpriseName,
			CreditCode: req.EnterpriseCreditCode,
			Industry:   req.EnterpriseIndustry,
			Scale:      req.EnterpriseScale,
			Address:    req.EnterpriseAddress,
		}
		if err := s.repo.CreateEnterprise(ent); err != nil {
			return nil, errcode.ErrInternal
		}
	case "carrier":
		carrier := &model.Carrier{
			UserID: user.ID,
			Name:   req.CarrierName,
			Type:   req.CarrierType,
			Area:   req.CarrierArea,
		}
		if err := s.repo.CreateCarrier(carrier); err != nil {
			return nil, errcode.ErrInternal
		}
	}
	token, _ := middleware.GenerateToken(s.cfg, user.ID, user.Role)
	info := toUserInfo(user)
	if req.Role == "enterprise" {
		info.CreditCode = req.EnterpriseCreditCode
	}
	return &dto.LoginResponse{Token: token, User: info}, nil
}

func (s *AuthService) Login(req *dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByCredential(req.Credential, req.Role)
	if err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("账号或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("账号或密码错误")
	}
	token, _ := middleware.GenerateToken(s.cfg, user.ID, user.Role)
	info := toUserInfo(user)
	if user.Role == "enterprise" {
		ent, err := s.repo.FindEnterpriseByUserID(user.ID)
		if err != nil {
			slog.Warn("failed to load enterprise credit_code", "user_id", user.ID, "error", err)
		} else if ent != nil {
			info.CreditCode = ent.CreditCode
		}
	}
	return &dto.LoginResponse{Token: token, User: info}, nil
}

func (s *AuthService) GetMe(userID uint) (*dto.UserInfo, error) {
	user, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	info := toUserInfo(user)
	if user.Role == "enterprise" {
		ent, err := s.repo.FindEnterpriseByUserID(user.ID)
		if err != nil {
			slog.Warn("failed to load enterprise credit_code", "user_id", user.ID, "error", err)
		} else if ent != nil {
			info.CreditCode = ent.CreditCode
		}
	}
	return &info, nil
}

func toUserInfo(u *model.User) dto.UserInfo {
	return dto.UserInfo{ID: u.ID, Role: u.Role}
}
