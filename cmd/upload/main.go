package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/database"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/internal/storage"
	"innovation-incubation-platform-backend/pkg/fileparser"
)

func main() {
	filePath := flag.String("file", "", "要上传的文件路径")
	userID := flag.Uint("user", 0, "上传者用户ID（政务账号的ID）")
	flag.Parse()

	if *filePath == "" || *userID == 0 {
		fmt.Fprintln(os.Stderr, "用法: go run ./cmd/upload/ --file <文件路径> --user <用户ID>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.MustLoad("config/config.yaml")
	db := database.MustInit(cfg)

	// 启动文件解析微服务（非阻断）
	ctx := context.Background()
	sidecar := fileparser.NewSidecar(cfg.FileParser)
	if err := sidecar.Start(ctx); err != nil {
		slog.Warn("file parser sidecar not available, falling back to local", "error", err)
	} else {
		defer sidecar.Stop()
	}

	f, err := os.Open(*filePath)
	if err != nil {
		slog.Error("打开文件失败", "error", err)
		os.Exit(1)
	}
	defer f.Close()

	stat, _ := f.Stat()
	fileName := stat.Name()
	mimeType := detectMimeType(fileName)

	fileRepo := repository.NewFileRepo(db)
	fileStorage, err := storage.NewLocalFileStorage(cfg.Upload.Dir)
	if err != nil {
		slog.Error("初始化文件存储失败", "error", err)
		os.Exit(1)
	}
	fileSvc := service.NewFileService(fileStorage, fileRepo, cfg)

	result, err := fileSvc.Upload(ctx, f, fileName, mimeType, stat.Size(), *userID)
	if err != nil {
		slog.Error("上传失败", "error", err)
		os.Exit(1)
	}

	fmt.Printf("上传成功\n")
	fmt.Printf("  文件ID: %d\n", result.ID)
	fmt.Printf("  文件名: %s\n", result.Filename)
	fmt.Printf("  大小: %d bytes\n", result.Size)
}

func detectMimeType(name string) string {
	ext := ""
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			ext = name[i:]
			break
		}
	}
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".txt":
		return "text/plain"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream"
	}
}
