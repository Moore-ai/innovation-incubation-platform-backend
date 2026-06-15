package service

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/dto"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/pkg/errcode"
	"innovation-incubation-platform-backend/internal/repository"
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
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errcode.ErrInternal
	}
	user := &model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         req.Role,
		Phone:        req.Phone,
		Email:        req.Email,
	}
	err = s.repo.CreateUser(user)
	if err != nil {
		return nil, errcode.ErrDuplicate.WithMsg("用户已存在")
	}
	if req.Role == "enterprise" {
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
	} else if req.Role == "carrier" {
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
	return &dto.LoginResponse{Token: token, User: toUserInfo(user)}, nil
}

func (s *AuthService) Login(req *dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.repo.FindByUsername(req.Username)
	if err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("用户名或密码错误")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errcode.ErrUnauthorized.WithMsg("用户名或密码错误")
	}
	token, _ := middleware.GenerateToken(s.cfg, user.ID, user.Role)
	return &dto.LoginResponse{Token: token, User: toUserInfo(user)}, nil
}

func (s *AuthService) GetMe(userID uint) (*dto.UserInfo, error) {
	user, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	info := toUserInfo(user)
	return &info, nil
}

func toUserInfo(u *model.User) dto.UserInfo {
	return dto.UserInfo{ID: u.ID, Username: u.Username, Role: u.Role, Phone: u.Phone, Email: u.Email}
}
