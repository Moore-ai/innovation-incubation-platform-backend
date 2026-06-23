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

func (r *FileRepo) ListByUploader(userID uint, page, pageSize int) ([]model.File, int64, error) {
	var list []model.File
	var total int64
	q := r.db.Model(&model.File{}).Where("uploaded_by = ?", userID)
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *FileRepo) ListAll(page, pageSize int) ([]model.File, int64, error) {
	var list []model.File
	var total int64
	q := r.db.Model(&model.File{})
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *FileRepo) IsReferenced(fileID uint) bool {
	var count int64
	r.db.Model(&model.IncubationRecord{}).Where("agreement_file_id = ?", fileID).Count(&count)
	return count > 0
}

func (r *FileRepo) Delete(id uint) error {
	return r.db.Delete(&model.File{}, id).Error
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
