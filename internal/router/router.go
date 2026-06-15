package router

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/internal/pkg/response"
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

type Deps struct {
	Config             *config.Config
	Enforcer           *casbin.Enforcer
	AuthController     *controller.AuthController
	EnterpriseController *controller.EnterpriseController
	CarrierController    *controller.CarrierController
	GovernmentController *controller.GovernmentController
}

func RegisterRoutes(r *gin.Engine, deps *Deps) {
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CorsMiddleware())
	r.Use(gin.Recovery())

	r.GET("/api/v1/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	pub := r.Group("/api/v1")
	registerAuthRoutes(pub, deps)

	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(deps.Config.JWT))
	if deps.Enforcer != nil {
		api.Use(middleware.RbacMiddleware(deps.Enforcer))
	}
	registerProtectedAuthRoutes(api, deps)
	registerEnterpriseRoutes(api, deps)
	registerCarrierRoutes(api, deps)
	registerGovernmentRoutes(api, deps)
}

func registerAuthRoutes(rg *gin.RouterGroup, deps *Deps) {
	auth := rg.Group("/auth")
	if deps.AuthController != nil {
		auth.POST("/register", deps.AuthController.Register)
		auth.POST("/login", deps.AuthController.Login)
	}
}

func registerProtectedAuthRoutes(rg *gin.RouterGroup, deps *Deps) {
	if deps.AuthController != nil {
		rg.GET("/auth/me", deps.AuthController.GetMe)
	}
}

func registerEnterpriseRoutes(rg *gin.RouterGroup, deps *Deps) {
	if deps.EnterpriseController == nil {
		return
	}
	e := rg.Group("/enterprise")
	e.POST("/incubation", deps.EnterpriseController.ApplyIncubation)
	e.GET("/incubation/:id", deps.EnterpriseController.GetIncubation)
	e.GET("/incubation/list", deps.EnterpriseController.ListMyIncubation)
	e.POST("/changes", deps.EnterpriseController.ApplyChange)
	e.GET("/changes/:id", deps.EnterpriseController.GetChange)
	e.GET("/changes/list", deps.EnterpriseController.ListMyChanges)
	e.PUT("/changes/:id", deps.EnterpriseController.ReeditChange)
	e.GET("/policies", deps.EnterpriseController.ListPolicies)
	e.POST("/policies/:id/apply", deps.EnterpriseController.ApplyPolicy)
	e.GET("/applications/list", deps.EnterpriseController.ListMyApplications)
	e.GET("/policies/:id/recommend", deps.EnterpriseController.RecommendPolicy)
	e.POST("/applications/:id/prefill", deps.EnterpriseController.PrefillApplication)
}

func registerCarrierRoutes(rg *gin.RouterGroup, deps *Deps) {
	if deps.CarrierController == nil {
		return
	}
	c := rg.Group("/carrier")
	c.GET("/incubation/list", deps.CarrierController.ListPendingIncubations)
	c.POST("/incubation/:id/approve", deps.CarrierController.ReviewIncubation)
	c.POST("/incubation/:id/reject", deps.CarrierController.ReviewIncubation)
	c.POST("/incubation/:id/return", deps.CarrierController.ReviewIncubation)
	c.GET("/changes/list", deps.CarrierController.ListPendingChanges)
	c.POST("/changes/:id/approve", deps.CarrierController.ReviewChange)
	c.POST("/changes/:id/reject", deps.CarrierController.ReviewChange)
	c.POST("/changes/:id/return", deps.CarrierController.ReviewChange)
	c.PUT("/info", deps.CarrierController.UpdateInfo)
	c.GET("/info", deps.CarrierController.GetMyInfo)
	c.GET("/policies", deps.CarrierController.ListPolicies)
	c.POST("/policies/:id/apply", deps.CarrierController.ApplyPolicy)
	c.GET("/applications/enterprise", deps.CarrierController.ListEnterpriseApplications)
	c.POST("/applications/:id/review", deps.CarrierController.ReviewEnterpriseApplication)
	c.GET("/performances", deps.CarrierController.ListCampaigns)
	c.POST("/performances/:id/submit", deps.CarrierController.SubmitPerformance)
}

func registerGovernmentRoutes(rg *gin.RouterGroup, deps *Deps) {
	if deps.GovernmentController == nil {
		return
	}
	g := rg.Group("/gov")
	g.POST("/policies/templates", deps.GovernmentController.CreatePolicyTemplate)
	g.POST("/policies", deps.GovernmentController.PublishPolicy)
	g.GET("/policies/list", deps.GovernmentController.ListPolicies)
	g.GET("/enterprises", deps.GovernmentController.SearchEnterprises)
	g.GET("/enterprises/:id", deps.GovernmentController.GetEnterprise)
	g.PUT("/enterprises/:id", deps.GovernmentController.EditEnterprise)
	g.GET("/carriers", deps.GovernmentController.SearchCarriers)
	g.POST("/applications/:id/review", deps.GovernmentController.ReviewPolicyApplication)
	g.GET("/applications/list", deps.GovernmentController.ListPolicyApplications)
	g.POST("/performances/templates", deps.GovernmentController.CreatePerformanceTemplate)
	g.POST("/performances/campaigns", deps.GovernmentController.StartCampaign)
	g.GET("/performances/submissions", deps.GovernmentController.ListSubmissions)
	g.POST("/performances/:id/score", deps.GovernmentController.ScoreSubmission)
}
