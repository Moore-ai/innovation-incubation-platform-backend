package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ReadSeekCloser 组合 io.ReadSeeker 和 io.Closer，供 http.ServeContent 使用
type ReadSeekCloser interface {
	io.ReadSeeker
	io.Closer
}

type Storage interface {
	Save(ctx context.Context, path string, reader io.Reader) error
	Open(ctx context.Context, path string) (ReadSeekCloser, error)
	Delete(ctx context.Context, path string) error
}

type LocalFileStorage struct {
	RootDir string
}

func NewLocalFileStorage(rootDir string) (*LocalFileStorage, error) {
	if rootDir == "" {
		return nil, fmt.Errorf("storage root directory cannot be empty")
	}
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}
	return &LocalFileStorage{RootDir: abs}, nil
}

func (s *LocalFileStorage) resolve(path string) (string, error) {
	full := filepath.Join(s.RootDir, path)
	root := filepath.Clean(s.RootDir) + string(filepath.Separator)
	if !strings.HasPrefix(strings.ToLower(filepath.Clean(full)), strings.ToLower(root)) &&
		!strings.EqualFold(filepath.Clean(full), filepath.Clean(s.RootDir)) {
		return "", fmt.Errorf("path traversal denied: %s", path)
	}
	return full, nil
}

func (s *LocalFileStorage) Save(ctx context.Context, path string, reader io.Reader) error {
	fullPath, err := s.resolve(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	// Write to temp then rename for crash atomicity
	tmpPath := fullPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(f, reader); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, fullPath)
}

func (s *LocalFileStorage) Open(ctx context.Context, path string) (ReadSeekCloser, error) {
	fullPath, err := s.resolve(path)
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

func (s *LocalFileStorage) Delete(ctx context.Context, path string) error {
	fullPath, err := s.resolve(path)
	if err != nil {
		return err
	}
	return os.Remove(fullPath)
}
