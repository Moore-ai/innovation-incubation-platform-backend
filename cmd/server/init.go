package main

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	"gorm.io/gorm"
)

type repositories struct {
	auth   *repository.AuthRepo
	ent    *repository.EnterpriseRepo
	carrier *repository.CarrierRepo
	gov    *repository.GovernmentRepo
	common *repository.CommonRepo
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
}

func initRepositories(db *gorm.DB) *repositories {
	return &repositories{
		auth:   repository.NewAuthRepo(db),
		ent:    repository.NewEnterpriseRepo(db),
		carrier: repository.NewCarrierRepo(db),
		gov:    repository.NewGovernmentRepo(db),
		common: repository.NewCommonRepo(db),
	}
}

func initServices(r *repositories, cfg *config.Config, db *gorm.DB) *services {
	return &services{
		auth:    service.NewAuthService(r.auth, cfg.JWT),
		ent:     service.NewEnterpriseService(r.ent, r.common, db),
		carrier: service.NewCarrierService(r.carrier, r.common, db),
		gov:     service.NewGovernmentService(r.gov, db),
	}
}

func initControllers(s *services) *controllers {
	return &controllers{
		auth:    controller.NewAuthController(s.auth),
		ent:     controller.NewEnterpriseController(s.ent),
		carrier: controller.NewCarrierController(s.carrier),
		gov:     controller.NewGovernmentController(s.gov),
	}
}
