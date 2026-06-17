package repository

import (
	"innovation-incubation-platform-backend/internal/model"
	"gorm.io/gorm"
)

type FileRepo struct {
	db *gorm.DB
}

func NewFileRepo(db *gorm.DB) *FileRepo {
	return &FileRepo{db: db}
}

func (r *FileRepo) Create(f *model.File) error {
	return r.db.Create(f).Error
}

func (r *FileRepo) FindByID(id uint) (*model.File, error) {
	var f model.File
	err := r.db.First(&f, id).Error
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *FileRepo) CheckFileAccess(fileID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.IncubationRecord{}).
		Joins("JOIN enterprises ON enterprises.id = incubation_records.enterprise_id").
		Where("incubation_records.agreement_file_id = ? AND enterprises.user_id = ?", fileID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	err = r.db.Model(&model.IncubationRecord{}).
		Joins("JOIN carriers ON carriers.id = incubation_records.carrier_id").
		Where("incubation_records.agreement_file_id = ? AND carriers.user_id = ?", fileID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
