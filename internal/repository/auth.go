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

func (r *AuthRepo) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
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

func (r *AuthRepo) CreateEnterprise(ent *model.Enterprise) error {
	return r.db.Create(ent).Error
}

func (r *AuthRepo) CreateCarrier(carrier *model.Carrier) error {
	return r.db.Create(carrier).Error
}
