package main

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"gorm.io/gorm"
)

type repositories struct {
	auth    *repository.AuthRepo
	ent     *repository.EnterpriseRepo
	carrier  *repository.CarrierRepo
	gov     *repository.GovernmentRepo
	common  *repository.CommonRepo
	file    *repository.FileRepo
}

type services struct {
	auth    *service.AuthService
	ent     *service.EnterpriseService
	carrier *service.CarrierService
	gov     *service.GovernmentService
}

type controllers struct {
	auth    *controller.AuthController
	ent     *controller.EnterpriseController
	carrier *controller.CarrierController
	gov     *controller.GovernmentController
	file    *controller.FileController
}

func initRepositories(db *gorm.DB) *repositories {
	return &repositories{
		auth:    repository.NewAuthRepo(db),
		ent:     repository.NewEnterpriseRepo(db),
		carrier:  repository.NewCarrierRepo(db),
		gov:     repository.NewGovernmentRepo(db),
		common:  repository.NewCommonRepo(db),
		file:    repository.NewFileRepo(db),
	}
}

func initServices(r *repositories, cfg *config.Config, db *gorm.DB) *services {
	aiClient := aiclient.NewAnthropicChatModel(aiclient.New(cfg.AI.Anthropic))
	aiSvc := service.NewAIService(aiClient, r.ent, r.gov, cfg)
	return &services{
		auth:    service.NewAuthService(r.auth, cfg.JWT),
		ent:     service.NewEnterpriseService(r.ent, r.common, db),
		carrier: service.NewCarrierService(r.carrier, r.common, db),
		gov:     service.NewGovernmentService(r.gov, db, aiSvc),
	}
}

func initControllers(r *repositories, s *services, cfg *config.Config) *controllers {
	return &controllers{
		auth:    controller.NewAuthController(s.auth),
		ent:     controller.NewEnterpriseController(s.ent),
		carrier: controller.NewCarrierController(s.carrier),
		gov:     controller.NewGovernmentController(s.gov),
		file:    controller.NewFileController(r.file, cfg),
	}
}
