package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/storage"
	"innovation-incubation-platform-backend/pkg/errcode"
	"innovation-incubation-platform-backend/pkg/fileparser"
)

type FileService struct {
	storage     storage.Storage
	repo        *repository.FileRepo
	allowedExts []string // 允许上传的扩展名（不含 . 前缀），空 = 无限制
}

func NewFileService(storage storage.Storage, repo *repository.FileRepo, cfg *config.Config) *FileService {
	allowedExts := cfg.Upload.AllowedExtensions
	exts := make([]string, len(allowedExts))
	for i, e := range allowedExts {
		exts[i] = strings.TrimPrefix(strings.ToLower(e), ".")
	}
	return &FileService{storage: storage, repo: repo, allowedExts: exts}
}

func (s *FileService) generatePath(ext string) string {
	id := uuid.New().String()
	shard := id[:2] + "/" + id[2:4]
	return fmt.Sprintf("%s/%s%s", shard, id, ext)
}

func (s *FileService) Upload(ctx context.Context, reader io.Reader, filename, mimeType string, size int64, userID uint) (*model.File, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if len(s.allowedExts) > 0 {
		if !slices.Contains(s.allowedExts, strings.TrimPrefix(ext, ".")) {
			return nil, errcode.ErrInvalidParams.WithMsg("不支持的文件类型，允许的扩展名：" + strings.Join(s.allowedExts, ", "))
		}
	}
	path := s.generatePath(ext)

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, errcode.ErrInternal
	}
	if size <= 0 {
		size = int64(len(data))
	}

	if err := s.storage.Save(ctx, path, bytes.NewReader(data)); err != nil {
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

	// 解析文件内容（尽力而为，失败不阻断）
	rawText, err := fileparser.Parse(bytes.NewReader(data), size, ext)
	if err != nil {
		slog.Warn("file parse failed", "file_id", f.ID, "ext", ext, "error", err)
	} else if rawText != "" {
		if err := s.repo.SetRawText(f.ID, rawText); err != nil {
			slog.Error("file set raw text failed", "file_id", f.ID, "error", err)
		}
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
	rc, err := s.storage.Open(ctx, f.StoragePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errcode.ErrNotFound
		}
		return nil, errcode.ErrInternal
	}
	return rc, nil
}

func (s *FileService) Delete(ctx context.Context, id uint) error {
	f, err := s.repo.FindByID(id)
	if err != nil {
		return errcode.ErrNotFound
	}
	if err := s.storage.Delete(ctx, f.StoragePath); err != nil {
		slog.Error("file delete failed", "path", f.StoragePath, "error", err)
		return errcode.ErrInternal
	}
	if err := s.repo.Delete(id); err != nil {
		slog.Error("file db record delete failed", "id", id, "error", err)
		return errcode.ErrInternal
	}
	return nil
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
