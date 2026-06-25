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
	Config                 *config.Config
	Enforcer               *casbin.Enforcer
	AuthController         *controller.AuthController
	EnterpriseController   *controller.EnterpriseController
	CarrierController      *controller.CarrierController
	GovernmentController   *controller.GovernmentController
	FileController         *controller.FileController
	NotificationController *controller.NotificationController
}

func RegisterRoutes(r *gin.Engine, deps *Deps) {
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.CorsMiddleware())
	r.Use(gin.Recovery())

	registerAuthRoutes(r, deps)
	registerUserRoutes(r, deps)
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
}

func registerUserRoutes(r *gin.Engine, deps *Deps) {
	if deps.AuthController == nil {
		return
	}
	u := protectedGroup(r, "/users", deps)
	u.GET("/me", deps.AuthController.GetMe)
}

func registerEnterpriseRoutes(r *gin.Engine, deps *Deps) {
	if deps.EnterpriseController == nil {
		return
	}
	e := protectedGroup(r, "/enterprise", deps)
	e.GET("/profile", deps.EnterpriseController.GetMyEnterpriseInfo)
	e.POST("/incubations", deps.EnterpriseController.ApplyIncubation)
	e.GET("/incubations/:id", deps.EnterpriseController.GetIncubation)
	e.GET("/incubations", deps.EnterpriseController.ListMyIncubation)
	e.POST("/changes", deps.EnterpriseController.ApplyChange)
	e.GET("/change-types", deps.EnterpriseController.ListChangeTypes)
	e.GET("/changes/:id", deps.EnterpriseController.GetChange)
	e.GET("/changes", deps.EnterpriseController.ListMyChanges)
	e.PUT("/changes/:id", deps.EnterpriseController.ReeditChange)
	e.GET("/policies", deps.EnterpriseController.ListPolicies)
	e.POST("/policies/:id/apply", deps.EnterpriseController.ApplyPolicy)
	e.GET("/applications", deps.EnterpriseController.ListMyApplications)
	e.POST("/account/deletion", deps.EnterpriseController.ApplyDeletion)
	e.GET("/carriers", deps.EnterpriseController.ListCarriers)
	e.GET("/carriers/:id", deps.EnterpriseController.GetCarrier)
	e.POST("/policies/:id/follow", deps.EnterpriseController.FollowPolicy)
	e.DELETE("/policies/:id/follow", deps.EnterpriseController.UnfollowPolicy)
	e.GET("/policies/follows", deps.EnterpriseController.ListFollowedPolicies)

	ai := r.Group("/api/v1/enterprise")
	ai.Use(middleware.AuthMiddleware(deps.Config.JWT))
	if deps.Enforcer != nil {
		ai.Use(middleware.RbacMiddleware(deps.Enforcer))
	}
	ai.Use(middleware.RouteRateLimit(5))
	ai.POST("/policies/search", deps.EnterpriseController.SearchPolicies)
	ai.GET("/policies/:id/recommend", deps.EnterpriseController.RecommendPolicy)
	ai.POST("/policies/:id/prefill", deps.EnterpriseController.PrefillApplication)
}

func registerCarrierRoutes(r *gin.Engine, deps *Deps) {
	if deps.CarrierController == nil {
		return
	}
	c := protectedGroup(r, "/carrier", deps)
	c.GET("/incubations/pending", deps.CarrierController.ListPendingIncubations)
	c.POST("/incubations/:id/review", deps.CarrierController.ReviewIncubation)
	c.POST("/incubations/:id/complete", deps.CarrierController.CompleteIncubation)
	c.GET("/changes", deps.CarrierController.ListPendingChanges)
	c.POST("/changes/:id/review", deps.CarrierController.ReviewChange)
	c.PUT("/info", deps.CarrierController.UpdateInfo)
	c.GET("/info", deps.CarrierController.GetMyInfo)
	c.GET("/policies", deps.CarrierController.ListPolicies)
	c.POST("/policies/:id/apply", deps.CarrierController.ApplyPolicy)
	c.GET("/applications", deps.CarrierController.ListEnterpriseApplications)
	c.POST("/applications/:id/review", deps.CarrierController.ReviewEnterpriseApplication)
	c.GET("/performances", deps.CarrierController.ListCampaigns)
	c.POST("/account/deletion", deps.CarrierController.ApplyDeletion)
	c.POST("/performances/:id/submit", deps.CarrierController.SubmitPerformance)
}

func registerGovernmentRoutes(r *gin.Engine, deps *Deps) {
	if deps.GovernmentController == nil {
		return
	}
	g := protectedGroup(r, "/gov", deps)
	g.POST("/policies", deps.GovernmentController.PublishPolicy)
	g.GET("/policies", deps.GovernmentController.ListPolicies)
	g.PUT("/policies/:id", deps.GovernmentController.UpdatePolicy)
	g.GET("/enterprises", deps.GovernmentController.SearchEnterprises)
	g.GET("/enterprises/:id", deps.GovernmentController.GetEnterprise)
	g.PUT("/enterprises/:id", deps.GovernmentController.EditEnterprise)
	g.DELETE("/enterprises/:id", deps.GovernmentController.DeleteEnterprise)
	g.DELETE("/carriers/:id", deps.GovernmentController.DeleteCarrier)
	g.GET("/carriers", deps.GovernmentController.SearchCarriers)
	g.POST("/applications/:id/review", deps.GovernmentController.ReviewPolicyApplication)
	g.GET("/applications", deps.GovernmentController.ListPolicyApplications)
	g.POST("/performances/templates", deps.GovernmentController.CreatePerformanceTemplate)
	g.POST("/performances/campaigns", deps.GovernmentController.StartCampaign)
	g.GET("/performances/submissions", deps.GovernmentController.ListSubmissions)
	g.GET("/account/deletions", deps.GovernmentController.ListDeletionRequests)
	g.POST("/account/deletions/:id/review", deps.GovernmentController.ReviewDeletionRequest)
	g.POST("/performances/:id/score", deps.GovernmentController.ScoreSubmission)
	g.POST("/incubations/:id/complete", deps.GovernmentController.CompleteIncubation)
}

func registerFileRoutes(r *gin.Engine, deps *Deps) {
	if deps.FileController == nil {
		return
	}
	f := r.Group("/api/v1/files")
	f.Use(middleware.AuthMiddleware(deps.Config.JWT))
	f.Use(middleware.GlobalRateLimit())
	f.GET("/limit", deps.FileController.GetUploadLimit)
	f.POST("/upload", deps.FileController.Upload)
	f.GET("/:id/download", deps.FileController.Download)
	f.GET("", deps.FileController.ListFiles)
	f.DELETE("/:id", deps.FileController.DeleteFile)
}

func registerNotificationRoutes(r *gin.Engine, deps *Deps) {
	if deps.NotificationController == nil {
		return
	}
	n := r.Group("/api/v1/notifications")
	n.Use(middleware.AuthMiddleware(deps.Config.JWT))
	n.GET("", deps.NotificationController.List)
	n.GET("/stream", deps.NotificationController.Subscribe)
	n.PATCH("/read", deps.NotificationController.MarkRead)
}
