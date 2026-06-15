package main

import (
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	r := gin.Default()
	slog.Info("server starting", "port", cfg.Server.Port)
	if err := r.Run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
