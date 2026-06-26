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
	deletion    *repository.DeletionRepo
	policyFollow *repository.PolicyFollowRepo
}

type services struct {
	auth    *service.AuthService
	ent     *service.EnterpriseService
	ai      *service.AIService
	carrier *service.CarrierService
	gov     *service.GovernmentService
	notif   *service.NotificationService
	file    *service.FileService
	search  service.PolicySearch
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
		policyFollow: repository.NewPolicyFollowRepo(db),
	}
}

func initServices(r *repositories, cfg *config.Config, db *gorm.DB, hub *service.SSEHub) *services {
	aiClient := aiclient.New(cfg.AI.OpenAI.BaseURL, cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.Model, cfg.AI.OpenAI.TimeoutSeconds)
	aiSvc := service.NewAIService(aiClient, r.ent, r.gov, r.file, cfg)
	notifSvc := service.NewNotificationService(r.notif, hub)
	assigner := service.NewAssigner(r.common)

	fileStorage, err := storage.NewLocalFileStorage(cfg.Upload.Dir)
	if err != nil {
		slog.Error("failed to init file storage", "error", err)
		os.Exit(1)
	}
	fileSvc := service.NewFileService(fileStorage, r.file, cfg)

	searchSvc := service.NewStructuredSearch(aiSvc, db, cfg.Search)

	var embedClient *aiclient.EmbeddingClient
	if cfg.AI.Embedding.APIKey != "" {
		embedClient = aiclient.NewEmbeddingClient(cfg.AI.Embedding)
	}

	return &services{
		auth:    service.NewAuthService(r.auth, cfg.JWT),
		ent:     service.NewEnterpriseService(r.ent, r.carrier, r.common, db, notifSvc, assigner, r.policyFollow),
		ai:      aiSvc,
		carrier: service.NewCarrierService(r.carrier, r.common, db, notifSvc, assigner),
		gov:     service.NewGovernmentService(r.gov, r.deletion, r.policyFollow, db, aiSvc, notifSvc, r.file, embedClient),
		notif:   notifSvc,
		file:    fileSvc,
		search:  searchSvc,
	}
}

func initControllers(r *repositories, s *services, cfg *config.Config, hub *service.SSEHub) *controllers {
	return &controllers{
		auth:    controller.NewAuthController(s.auth),
		ent:     controller.NewEnterpriseController(s.ent, s.ai, s.search),
		carrier: controller.NewCarrierController(s.carrier),
		gov:     controller.NewGovernmentController(s.gov),
		file:    controller.NewFileController(s.file, cfg),
		notif:   controller.NewNotificationController(r.notif, hub, cfg),
	}
}
