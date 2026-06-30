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

func (r *AuthRepo) FindByCredential(credential, role string) (*model.User, error) {
	var user model.User
	if role == string(model.RoleEnterprise) {
		// 企业用户：先尝试信用代码，再尝试手机号
		err := r.db.Joins("JOIN enterprises ON enterprises.user_id = users.id").
			Where("enterprises.credit_code = ?", credential).First(&user).Error
		if err != nil {
			// 再用手机号查找
			err = r.db.Where("phone = ? AND role = ?", credential, role).First(&user).Error
			if err != nil {
				return nil, err
			}
		}
	} else {
		err := r.db.Where("phone = ? AND role = ?", credential, role).First(&user).Error
		if err != nil {
			return nil, err
		}
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

func (r *AuthRepo) CreateEnterprise(ent *model.Enterprise) error {
	return r.db.Create(ent).Error
}

func (r *AuthRepo) CreateCarrier(carrier *model.Carrier) error {
	return r.db.Create(carrier).Error
}
