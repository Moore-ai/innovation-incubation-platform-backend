package main

import (
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/database"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/router"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.MustLoad("config/config.yaml")
	db := database.MustInit(cfg)
	enforcer := middleware.MustInitEnforcer(db)

	repo := initRepositories(db)
	svc := initServices(repo, cfg, db)
	ctl := initControllers(svc)

	r := gin.New()
	router.RegisterRoutes(r, &router.Deps{
		Config:               cfg,
		Enforcer:             enforcer,
		AuthController:       ctl.auth,
		EnterpriseController: ctl.ent,
		CarrierController:    ctl.carrier,
		GovernmentController: ctl.gov,
	})

	slog.Info("server starting", "port", cfg.Server.Port)
	if err := r.Run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
