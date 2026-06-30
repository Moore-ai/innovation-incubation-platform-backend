package repository

import (
	"innovation-incubation-platform-backend/internal/model"

	"gorm.io/gorm"
)

type AuthRepo struct {
	db *gorm.DB
}

func NewAuthRepo(db *gorm.DB) *AuthRepo {
	return &AuthRepo{db: db}
}

func (r *AuthRepo) CreateUser(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *AuthRepo) FindByPhone(phone, role string) (*model.User, error) {
	var user model.User
	err := r.db.Where("phone = ? AND role = ?", phone, role).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepo) FindByCreditCode(creditCode string) (*model.User, error) {
	var user model.User
	err := r.db.Joins("JOIN enterprises ON enterprises.user_id = users.id").
		Where("enterprises.credit_code = ?", creditCode).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepo) FindByUserID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepo) FindEnterpriseByUserID(userID uint) (*model.Enterprise, error) {
	var ent model.Enterprise
	err := r.db.Where("user_id = ?", userID).First(&ent).Error
	if err != nil {
		return nil, err
	}
	return &ent, nil
}

func (r *AuthRepo) FindGovernmentByUserID(userID uint) (*model.Government, error) {
	var gov model.Government
	err := r.db.Where("user_id = ?", userID).First(&gov).Error
	if err != nil {
		return nil, err
	}
	return &gov, nil
}

func (r *AuthRepo) CreateEnterprise(ent *model.Enterprise) error {
	return r.db.Create(ent).Error
}

func (r *AuthRepo) CreateGovernment(gov *model.Government) error {
	return r.db.Create(gov).Error
}

func (r *AuthRepo) CreateCarrier(carrier *model.Carrier) error {
	return r.db.Create(carrier).Error
}
