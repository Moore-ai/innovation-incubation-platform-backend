package router

import (
	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/controller"
	"innovation-incubation-platform-backend/internal/middleware"
	"innovation-incubation-platform-backend/pkg/response"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
)

type Deps struct {
	Config               *config.Config
	Enforcer             *casbin.Enforcer
	AuthController       *controller.AuthController
	EnterpriseController *controller.EnterpriseController
	CarrierController    *controller.CarrierController
	GovernmentController *controller.GovernmentController
	FileController          *controller.FileController
	NotificationController *controller.NotificationController
}

func RegisterRoutes(r *gin.Engine, deps *Deps) {
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CorsMiddleware())
	r.Use(gin.Recovery())

	registerAuthRoutes(r, deps)
	registerEnterpriseRoutes(r, deps)
	registerCarrierRoutes(r, deps)
	registerGovernmentRoutes(r, deps)
	registerFileRoutes(r, deps)
	registerNotificationRoutes(r, deps)

	r.GET("/api/v1/health", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})
}

func protectedGroup(r *gin.Engine, prefix string, deps *Deps) *gin.RouterGroup {
	g := r.Group("/api/v1" + prefix)
	g.Use(middleware.AuthMiddleware(deps.Config.JWT))
	if deps.Enforcer != nil {
		g.Use(middleware.RbacMiddleware(deps.Enforcer))
	}
	g.Use(middleware.GlobalRateLimit())
	return g
}

func registerAuthRoutes(r *gin.Engine, deps *Deps) {
	if deps.AuthController == nil {
		return
	}
	pub := r.Group("/api/v1/auth")
	pub.Use(middleware.RouteRateLimit(10))
	pub.POST("/register", deps.AuthController.Register)
	pub.POST("/login", deps.AuthController.Login)

	me := protectedGroup(r, "/auth", deps)
	me.GET("/me", deps.AuthController.GetMe)
}

func registerEnterpriseRoutes(r *gin.Engine, deps *Deps) {
	if deps.EnterpriseController == nil {
		return
	}
	e := protectedGroup(r, "/enterprise", deps)
	e.GET("/my-info", deps.EnterpriseController.GetMyEnterpriseInfo)
	e.POST("/incubation", deps.EnterpriseController.ApplyIncubation)
	e.GET("/incubation/:id", deps.EnterpriseController.GetIncubation)
	e.GET("/incubation/list", deps.EnterpriseController.ListMyIncubation)
	e.POST("/changes", deps.EnterpriseController.ApplyChange)
	e.GET("/changes/types", deps.EnterpriseController.ListChangeTypes)
	e.GET("/changes/:id", deps.EnterpriseController.GetChange)
	e.GET("/changes/list", deps.EnterpriseController.ListMyChanges)
	e.PUT("/changes/:id", deps.EnterpriseController.ReeditChange)
	e.GET("/policies", deps.EnterpriseController.ListPolicies)
	e.POST("/policies/:id/apply", deps.EnterpriseController.ApplyPolicy)
	e.GET("/applications/list", deps.EnterpriseController.ListMyApplications)

	ai := r.Group("/api/v1/enterprise")
	ai.Use(middleware.AuthMiddleware(deps.Config.JWT))
	if deps.Enforcer != nil {
		ai.Use(middleware.RbacMiddleware(deps.Enforcer))
	}
	ai.Use(middleware.RouteRateLimit(5))
	ai.GET("/policies/:id/recommend", deps.EnterpriseController.RecommendPolicy)
	ai.POST("/policies/prefill", deps.EnterpriseController.PrefillApplication)
}

func registerCarrierRoutes(r *gin.Engine, deps *Deps) {
	if deps.CarrierController == nil {
		return
	}
	c := protectedGroup(r, "/carrier", deps)
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

func registerGovernmentRoutes(r *gin.Engine, deps *Deps) {
	if deps.GovernmentController == nil {
		return
	}
	g := protectedGroup(r, "/gov", deps)
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

func registerFileRoutes(r *gin.Engine, deps *Deps) {
	if deps.FileController == nil {
		return
	}
	f := r.Group("/api/v1/files")
	f.Use(middleware.AuthMiddleware(deps.Config.JWT))
	f.GET("/limit", deps.FileController.GetUploadLimit)
	f.POST("/upload", deps.FileController.Upload)
	f.GET("/:id/download", deps.FileController.Download)
}

func registerNotificationRoutes(r *gin.Engine, deps *Deps) {
	if deps.NotificationController == nil {
		return
	}
	n := r.Group("/api/v1/notifications")
	n.Use(middleware.AuthMiddleware(deps.Config.JWT))
	n.GET("/subscribe", deps.NotificationController.Subscribe)
	n.PATCH("/read", deps.NotificationController.MarkRead)
}
