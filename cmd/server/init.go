package main

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	agentpkg "innovation-incubation-platform-backend/internal/service/agent"
	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	agentbuiltin "innovation-incubation-platform-backend/internal/service/agent/tools/builtin"
	"innovation-incubation-platform-backend/internal/storage"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"log/slog"
	"os"

	"gorm.io/gorm"
)

type repositories struct {
	auth         *repository.AuthRepo
	ent          *repository.EnterpriseRepo
	carrier      *repository.CarrierRepo
	gov          *repository.GovernmentRepo
	common       *repository.CommonRepo
	file         *repository.FileRepo
	notif        *repository.NotificationRepo
	deletion     *repository.DeletionRepo
	policyFollow *repository.PolicyFollowRepo
	appeal       *repository.AppealRepo
	chat         *repository.ChatRepo
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
	test    *service.TestService
	appeal  *service.AppealService
	chat    *service.ChatService
}

type controllers struct {
	auth    *controller.AuthController
	ent     *controller.EnterpriseController
	carrier *controller.CarrierController
	gov     *controller.GovernmentController
	file    *controller.FileController
	notif   *controller.NotificationController
	test    *controller.TestController
	chat    *controller.ChatController
}

func initRepositories(db *gorm.DB) *repositories {
	return &repositories{
		auth:         repository.NewAuthRepo(db),
		ent:          repository.NewEnterpriseRepo(db),
		carrier:      repository.NewCarrierRepo(db),
		gov:          repository.NewGovernmentRepo(db),
		common:       repository.NewCommonRepo(db),
		file:         repository.NewFileRepo(db),
		notif:        repository.NewNotificationRepo(db),
		deletion:     repository.NewDeletionRepo(db),
		policyFollow: repository.NewPolicyFollowRepo(db),
		appeal:       repository.NewAppealRepo(db),
		chat:         repository.NewChatRepo(db),
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

	var embedClient *aiclient.EmbeddingClient
	if cfg.AI.Embedding.APIKey != "" {
		embedClient = aiclient.NewEmbeddingClient(cfg.AI.Embedding)
	}

	var searchSvc service.PolicySearch
	switch cfg.Search.Method {
	case "vector":
		if embedClient == nil {
			slog.Error("vector search requires embed client, falling back to structured")
			searchSvc = service.NewStructuredSearch(aiSvc, r.carrier, db, cfg.Search)
		} else {
			expander := service.NewQueryExpander(aiClient, cfg.Search.Vector.MQE.NQueries)
			var hydeGen *service.HyDEGenerator
			if cfg.Search.Vector.HyDE.Enabled && aiClient != nil {
				hydeGen = service.NewHyDEGenerator(aiClient, cfg.Search.Vector.HyDE)
			}
			searchSvc = service.NewVectorSearch(embedClient, aiSvc, expander, hydeGen, r.carrier, db, cfg.Search)
		}
	case "structured":
		searchSvc = service.NewStructuredSearch(aiSvc, r.carrier, db, cfg.Search)
	default:
		searchSvc = service.NewStructuredSearch(aiSvc, r.carrier, db, cfg.Search)
	}

	appealSvc := service.NewAppealService(r.appeal)

	// Agent
	agentToolRegistry := agenttools.NewToolRegistry()
	agentToolRegistry.Register(agentbuiltin.NewSearchPolicy(searchSvc))
	agentToolRegistry.Register(agentbuiltin.NewQueryEnterpriseInfo(r.ent))
	agentToolRegistry.Register(agentbuiltin.NewQueryAppeal(r.appeal))
	agentToolRegistry.Register(agentbuiltin.NewQueryPolicyFollow(r.policyFollow))
	agentToolRegistry.Register(agentbuiltin.NewQueryIncubationRecords(r.ent))
	agentToolRegistry.Register(agentbuiltin.NewQueryChangeHistory(r.ent))
	agentToolRegistry.Register(agentbuiltin.NewQueryMyPolicyApplications(r.ent))
	agentToolRegistry.Register(agentbuiltin.NewQueryMyCarrierInfo(r.carrier))
	agentToolRegistry.Register(agentbuiltin.NewQueryPendingIncubations(r.carrier))
	agentToolRegistry.Register(agentbuiltin.NewQueryPendingChanges(r.carrier))
	agentToolRegistry.Register(agentbuiltin.NewQueryEnterpriseApplications(r.carrier))
	agentToolRegistry.Register(agentbuiltin.NewQueryPerformanceCampaigns(r.carrier))
	agentToolRegistry.Register(agentbuiltin.NewQueryApplicationsByStatus(r.carrier))
	agentToolRegistry.Register(agentbuiltin.NewQueryPolicyDetail(r.gov))
	agentToolRegistry.Register(agentbuiltin.NewQueryMyFiles(r.file))

	agentWorkingMem := agentmemory.NewWorkingMemory(r.chat, cfg.Agent.WorkingMemory.PageSize)
	agentSemanticMem := agentmemory.NewSemanticMemory(r.chat, embedClient, cfg.Agent.Memory.SemanticLimit)
	agentMemMgr := agentmemory.NewMemoryManager(agentWorkingMem, agentSemanticMem, r.chat, embedClient, cfg.Agent)

	agentReflect := agentpkg.NewReflectChecker(embedClient, agentToolRegistry, cfg.Agent.Reflect)
	agentEngine := agentpkg.NewEngine(aiClient, agentToolRegistry, agentMemMgr, agentReflect, cfg.Agent)

	chatSvc := service.NewChatService(agentEngine, r.chat, agentMemMgr, aiClient, cfg.Agent)

	return &services{
		auth:    service.NewAuthService(r.auth, cfg.JWT),
		ent:     service.NewEnterpriseService(r.ent, r.carrier, r.common, db, notifSvc, assigner, r.policyFollow),
		ai:      aiSvc,
		carrier: service.NewCarrierService(r.carrier, r.common, db, notifSvc, assigner),
		gov:     service.NewGovernmentService(r.gov, r.deletion, r.policyFollow, db, aiSvc, notifSvc, r.file, embedClient),
		notif:   notifSvc,
		file:    fileSvc,
		search:  searchSvc,
		test:    service.NewTestService(aiClient, embedClient),
		appeal:  appealSvc,
		chat:    chatSvc,
	}
}

func initControllers(r *repositories, s *services, cfg *config.Config, hub *service.SSEHub) *controllers {
	return &controllers{
		auth:    controller.NewAuthController(s.auth),
		ent:     controller.NewEnterpriseController(s.ent, s.ai, s.search, s.appeal),
		carrier: controller.NewCarrierController(s.carrier, s.appeal, s.search),
		gov:     controller.NewGovernmentController(s.gov, s.appeal),
		file:    controller.NewFileController(s.file, cfg),
		notif:   controller.NewNotificationController(r.notif, hub, cfg),
		test:    controller.NewTestController(s.test),
		chat:    controller.NewChatController(s.chat, cfg),
	}
}
