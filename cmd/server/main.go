package main

import (
	"log/slog"
	"os"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/pkg/aiclient"
	"innovation-incubation-platform-backend/internal/pkg/database"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/router"
	"innovation-incubation-platform-backend/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	db, err := database.NewDB(cfg.DB)
	if err != nil {
		slog.Error("failed to connect database", "error", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Enterprise{},
		&model.Carrier{},
		&model.IncubationRecord{},
		&model.MajorChange{},
		&model.PolicyTemplate{},
		&model.Policy{},
		&model.PolicyApplication{},
		&model.Approval{},
		&model.PerformanceTemplate{},
		&model.PerformanceCampaign{},
		&model.PerformanceSubmission{},
	); err != nil {
		slog.Error("failed to auto migrate", "error", err)
		os.Exit(1)
	}

	enforcer, err := middleware.NewEnforcer(db)
	if err != nil {
		slog.Error("failed to init casbin enforcer", "error", err)
		os.Exit(1)
	}
	middleware.SeedPolicies(enforcer)

	aiClnt := aiclient.New(cfg.AI.Anthropic)
	_ = aiClnt

	authRepo := repository.NewAuthRepo(db)
	entRepo := repository.NewEnterpriseRepo(db)
	carrierRepo := repository.NewCarrierRepo(db)
	govRepo := repository.NewGovernmentRepo(db)
	commonRepo := repository.NewCommonRepo(db)

	authSvc := service.NewAuthService(authRepo, cfg.JWT)
	entSvc := service.NewEnterpriseService(entRepo, commonRepo, db)
	carrierSvc := service.NewCarrierService(carrierRepo, commonRepo, db)
	govSvc := service.NewGovernmentService(govRepo, db)

	authCtl := controller.NewAuthController(authSvc)
	entCtl := controller.NewEnterpriseController(entSvc)
	carrierCtl := controller.NewCarrierController(carrierSvc)
	govCtl := controller.NewGovernmentController(govSvc)

	r := gin.New()
	deps := &router.Deps{
		Config:               cfg,
		Enforcer:             enforcer,
		AuthController:       authCtl,
		EnterpriseController: entCtl,
		CarrierController:    carrierCtl,
		GovernmentController: govCtl,
	}
	router.RegisterRoutes(r, deps)

	slog.Info("server starting", "port", cfg.Server.Port)
	if err := r.Run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
