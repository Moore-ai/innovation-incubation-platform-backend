package main

import (
	"log/slog"
	"os"
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/internal/storage"
	"innovation-incubation-platform-backend/pkg/aiclient"

	"gorm.io/gorm"
)

type repositories struct {
	auth    *repository.AuthRepo
	ent     *repository.EnterpriseRepo
	carrier *repository.CarrierRepo
	gov     *repository.GovernmentRepo
	common  *repository.CommonRepo
	file    *repository.FileRepo
	notif   *repository.NotificationRepo
	deletion *repository.DeletionRepo
}

type services struct {
	auth    *service.AuthService
	ent     *service.EnterpriseService
	ai      *service.AIService
	carrier *service.CarrierService
	gov     *service.GovernmentService
	notif   *service.NotificationService
	file    *service.FileService
}

type controllers struct {
	auth    *controller.AuthController
	ent     *controller.EnterpriseController
	carrier *controller.CarrierController
	gov     *controller.GovernmentController
	file    *controller.FileController
	notif   *controller.NotificationController
}

func initRepositories(db *gorm.DB) *repositories {
	return &repositories{
		auth:    repository.NewAuthRepo(db),
		ent:     repository.NewEnterpriseRepo(db),
		carrier: repository.NewCarrierRepo(db),
		gov:     repository.NewGovernmentRepo(db),
		common:  repository.NewCommonRepo(db),
		file:    repository.NewFileRepo(db),
		notif:   repository.NewNotificationRepo(db),
		deletion: repository.NewDeletionRepo(db),
	}
}

func initServices(r *repositories, cfg *config.Config, db *gorm.DB, hub *service.SSEHub) *services {
	aiClient := aiclient.NewAnthropicChatModel(aiclient.New(cfg.AI.Anthropic))
	aiSvc := service.NewAIService(aiClient, r.ent, r.gov, cfg)
	notifSvc := service.NewNotificationService(r.notif, hub)

	fileStorage, err := storage.NewLocalFileStorage(cfg.Upload.Dir)
	if err != nil {
		slog.Error("failed to init file storage", "error", err)
		os.Exit(1)
	}
	fileSvc := service.NewFileService(fileStorage, r.file, cfg)

	return &services{
		auth:    service.NewAuthService(r.auth, cfg.JWT),
		ent:     service.NewEnterpriseService(r.ent, r.common, db, notifSvc),
		ai:      aiSvc,
		carrier: service.NewCarrierService(r.carrier, r.common, db, notifSvc),
		gov:     service.NewGovernmentService(r.gov, r.deletion, db, aiSvc, notifSvc),
		notif:   notifSvc,
		file:    fileSvc,
	}
}

func initControllers(r *repositories, s *services, cfg *config.Config, hub *service.SSEHub) *controllers {
	return &controllers{
		auth:    controller.NewAuthController(s.auth),
		ent:     controller.NewEnterpriseController(s.ent, s.ai),
		carrier: controller.NewCarrierController(s.carrier),
		gov:     controller.NewGovernmentController(s.gov),
		file:    controller.NewFileController(s.file, cfg),
		notif:   controller.NewNotificationController(r.notif, hub, cfg),
	}
}
