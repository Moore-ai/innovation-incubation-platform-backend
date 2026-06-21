package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/google/uuid"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/storage"
	"innovation-incubation-platform-backend/pkg/errcode"
)

type FileService struct {
	storage storage.Storage
	repo    *repository.FileRepo
}

func NewFileService(storage storage.Storage, repo *repository.FileRepo) *FileService {
	return &FileService{storage: storage, repo: repo}
}

func (s *FileService) generatePath(ext string) string {
	id := uuid.New().String()
	shard := id[:2] + "/" + id[2:4]
	return fmt.Sprintf("%s/%s%s", shard, id, ext)
}

func (s *FileService) Upload(ctx context.Context, reader io.Reader, filename, mimeType string, size int64, userID uint) (*model.File, error) {
	ext := filepath.Ext(filename)
	path := s.generatePath(ext)

	if err := s.storage.Save(ctx, path, reader); err != nil {
		return nil, errcode.ErrInternal
	}

	f := &model.File{
		Filename:    filename,
		MimeType:    mimeType,
		Size:        size,
		StoragePath: path,
		UploadedBy:  userID,
	}
	if err := s.repo.Create(f); err != nil {
		s.storage.Delete(ctx, path)
		return nil, errcode.ErrInternal
	}
	return f, nil
}

func (s *FileService) GetMeta(id uint) (*model.File, error) {
	return s.repo.FindByID(id)
}

func (s *FileService) Open(ctx context.Context, id uint) (storage.ReadSeekCloser, error) {
	f, err := s.repo.FindByID(id)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return s.storage.Open(ctx, f.StoragePath)
}

func (s *FileService) Delete(ctx context.Context, id uint) error {
	f, err := s.repo.FindByID(id)
	if err != nil {
		return errcode.ErrNotFound
	}
	if err := s.storage.Delete(ctx, f.StoragePath); err != nil {
		return errcode.ErrInternal
	}
	return s.repo.Delete(id)
}

func (s *FileService) ListByUploader(userID uint, page, pageSize int) ([]model.File, int64, error) {
	return s.repo.ListByUploader(userID, page, pageSize)
}

func (s *FileService) ListAll(page, pageSize int) ([]model.File, int64, error) {
	return s.repo.ListAll(page, pageSize)
}

func (s *FileService) IsReferenced(id uint) bool {
	return s.repo.IsReferenced(id)
}

func (s *FileService) HasFileAccess(fileID, userID uint) (bool, error) {
	return s.repo.CheckFileAccess(fileID, userID)
}
