package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalFileStorage_SaveAndOpen(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalFileStorage(dir)

	content := "hello storage"
	path := "test/hello.txt"
	ctx := context.Background()

	if err := s.Save(ctx, path, strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}

	rc, err := s.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("got %q, want %q", string(data), content)
	}
}

func TestLocalFileStorage_Delete(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalFileStorage(dir)
	ctx := context.Background()

	if err := s.Save(ctx, "del.txt", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "del.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "del.txt")); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
}

func TestLocalFileStorage_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalFileStorage(dir)
	ctx := context.Background()

	err := s.Save(ctx, "../../evil.txt", strings.NewReader("x"))
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestLocalFileStorage_OpenNonExistent(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalFileStorage(dir)
	_, err := s.Open(context.Background(), "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLocalFileStorage_DeleteNonExistent(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalFileStorage(dir)
	err := s.Delete(context.Background(), "nonexistent.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLocalFileStorage_OpenPathTraversal(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalFileStorage(dir)
	_, err := s.Open(context.Background(), "../../evil.txt")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}
