package main

import (
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	r := gin.Default()

	r.GET("/api/v1/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	slog.Info("server starting", "port", cfg.Server.Port)
	if err := r.Run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
