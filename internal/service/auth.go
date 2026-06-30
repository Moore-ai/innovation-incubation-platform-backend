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
	if !model.UserRole(req.Role).IsValid() {
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
	case string(model.UserRoleEnterprise):
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
	case string(model.UserRoleCarrier):
		carrier := &model.Carrier{
			UserID: user.ID,
			Name:   req.CarrierName,
			Type:   req.CarrierType,
			Area:   req.CarrierArea,
		}
		if err := s.repo.CreateCarrier(carrier); err != nil {
			return nil, errcode.ErrInternal
		}
	case string(model.UserRoleGovernment):
		gov := &model.Government{
			UserID:     user.ID,
			Name:       req.GovName,
			Department: req.GovDepartment,
		}
		if err := s.repo.CreateGovernment(gov); err != nil {
			return nil, errcode.ErrInternal
		}
	}
	token, _ := middleware.GenerateToken(s.cfg, user.ID, user.Role)
	info := toUserInfo(user)
	if req.Role == string(model.UserRoleEnterprise) {
		info.CreditCode = req.EnterpriseCreditCode
	}
	return &dto.LoginResponse{Token: token, User: info}, nil
}

func (s *AuthService) Login(req *dto.LoginRequest) (*dto.LoginResponse, error) {
	var user *model.User
	var err error

	switch req.Role {
	case string(model.UserRoleEnterprise):
		if req.CreditCode == "" {
			return nil, errcode.ErrInvalidParams.WithMsg("企业登录需提供信用代码")
		}
		user, err = s.repo.FindByCreditCode(req.CreditCode)
	case string(model.UserRoleCarrier):
		if req.Phone == "" {
			return nil, errcode.ErrInvalidParams.WithMsg("载体登录需提供手机号")
		}
		user, err = s.repo.FindByPhone(req.Phone, req.Role)
	default:
		if req.Phone == "" {
			return nil, errcode.ErrInvalidParams.WithMsg("手机号不能为空")
		}
		user, err = s.repo.FindByPhone(req.Phone, req.Role)
	}
	if err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("账号或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("账号或密码错误")
	}
	token, _ := middleware.GenerateToken(s.cfg, user.ID, user.Role)
	info := toUserInfo(user)
	if user.Role == string(model.UserRoleEnterprise) {
		ent, err := s.repo.FindEnterpriseByUserID(user.ID)
		if err != nil {
			slog.Warn("failed to load enterprise credit_code", "user_id", user.ID, "error", err)
		} else if ent != nil {
			info.CreditCode = ent.CreditCode
		}
	} else if user.Role == string(model.UserRoleGovernment) {
		gov, err := s.repo.FindGovernmentByUserID(user.ID)
		if err != nil {
			slog.Warn("failed to load government info", "user_id", user.ID, "error", err)
		} else if gov != nil {
			info.Name = gov.Name
			info.Department = gov.Department
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
	if user.Role == string(model.UserRoleEnterprise) {
		ent, err := s.repo.FindEnterpriseByUserID(user.ID)
		if err != nil {
			slog.Warn("failed to load enterprise credit_code", "user_id", user.ID, "error", err)
		} else if ent != nil {
			info.CreditCode = ent.CreditCode
		}
	} else if user.Role == string(model.UserRoleGovernment) {
		gov, err := s.repo.FindGovernmentByUserID(user.ID)
		if err != nil {
			slog.Warn("failed to load government info", "user_id", user.ID, "error", err)
		} else if gov != nil {
			info.Name = gov.Name
			info.Department = gov.Department
		}
	}
	return &info, nil
}

func toUserInfo(u *model.User) dto.UserInfo {
	return dto.UserInfo{ID: u.ID, Role: u.Role}
}
