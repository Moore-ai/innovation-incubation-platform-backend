package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/database"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/router"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/fileparser"

	"github.com/gin-gonic/gin"
)

func initLog(cfg *config.Config) {
	if !cfg.Log.Enabled {
		return
	}
	f, err := database.InitFileLogger("log")
	if err != nil {
		slog.Error("failed to init file logger", "error", err)
		return
	}
	slog.Info("file logging enabled", "file", f.Name())
}

func main() {
	cfg := config.MustLoad("config/config.yaml")
	gin.SetMode(cfg.Server.Mode)
	initLog(cfg)
	db := database.MustInit(cfg)
	database.MustNewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	middleware.InitRateLimit(&cfg.RateLimit)
	cfg.Upload.Init()
	enforcer, _ := middleware.NewEnforcer(db) // 临时禁用 RBAC

	hub := service.NewSSEHub(cfg.Notification.MaxConnsPerUser)
	repo := initRepositories(db)

	// 启动 file parser sidecar（非阻断）
	sidecar := fileparser.NewSidecar(cfg.FileParser)
	if err := sidecar.Start(context.Background()); err != nil {
		slog.Warn("file parser sidecar not available, falling back to local", "error", err)
	} else {
		defer sidecar.Stop()
	}

	svc := initServices(repo, cfg, db, hub)
	ctl := initControllers(repo, svc, cfg, hub)

	r := gin.New()
	router.RegisterRoutes(r, &router.Deps{
		Config:                 cfg,
		Enforcer:               enforcer,
		AuthController:         ctl.auth,
		EnterpriseController:   ctl.ent,
		CarrierController:      ctl.carrier,
		GovernmentController:   ctl.gov,
		FileController:         ctl.file,
		NotificationController: ctl.notif,
		TestController:         ctl.test,
	})

	slog.Info("server starting", "port", cfg.Server.Port)
	if err := r.Run(fmt.Sprintf(":%d", cfg.Server.Port)); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
