package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peizhengma/biography-v2/internal/api/admin"
	"github.com/peizhengma/biography-v2/internal/api/middleware"
	"github.com/peizhengma/biography-v2/internal/api/realtime"
	"github.com/peizhengma/biography-v2/internal/api/user"
	"github.com/peizhengma/biography-v2/internal/config"
	"github.com/peizhengma/biography-v2/internal/storage/postgres"
)

// NewRouter 创建路由
func NewRouter(cfg *config.Config, db *postgres.DB) http.Handler {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 中间件
	r.Use(gin.Recovery())
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "biography-v2",
		})
	})

	// API 路由组
	api := r.Group("/api")

	// 公开路由
	api.POST("/auth/register", user.Register)
	api.POST("/auth/login", user.Login)

	// 用户端路由（需要 JWT 认证）
	userRoutes := api.Group("")
	userRoutes.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		// 用户信息
		userRoutes.GET("/user/profile", user.GetProfile)
		userRoutes.PUT("/user/profile", user.UpdateProfile)
		userRoutes.PUT("/user/password", user.ChangePassword)

		// 对话
		userRoutes.GET("/conversations", user.ListConversations)
		userRoutes.POST("/conversations", user.CreateConversation)
		userRoutes.GET("/conversations/:id", user.GetConversation)
		userRoutes.GET("/conversations/:id/messages", user.GetMessages)

		// 回忆录
		userRoutes.GET("/memoirs", user.ListMemoirs)
		userRoutes.GET("/memoirs/:id", user.GetMemoir)

		// 话题
		userRoutes.GET("/topics", user.GetTopicOptions)
	}

	// WebSocket 实时对话（需要 JWT 认证，通过 query param）
	api.GET("/realtime/dialog", realtime.HandleDialog)

	// 管理端路由（需要 Admin API Key）
	adminRoutes := api.Group("/admin")
	adminRoutes.Use(middleware.AdminAuth(cfg.AdminAPIKey))
	{
		// 用户管理
		adminRoutes.GET("/users", admin.ListUsers)
		adminRoutes.GET("/users/:id", admin.GetUser)
		adminRoutes.PUT("/users/:id", admin.UpdateUser)
		adminRoutes.DELETE("/users/:id", admin.DeleteUser)

		// 对话管理
		adminRoutes.GET("/conversations", admin.ListConversations)
		adminRoutes.GET("/conversations/:id", admin.GetConversation)

		// 回忆录管理
		adminRoutes.GET("/memoirs", admin.ListMemoirs)
		adminRoutes.PUT("/memoirs/:id", admin.UpdateMemoir)
		adminRoutes.DELETE("/memoirs/:id", admin.DeleteMemoir)
		adminRoutes.POST("/memoirs/:id/regenerate", admin.RegenerateMemoir)

		// 话题管理
		adminRoutes.GET("/topics", admin.ListTopics)
		adminRoutes.POST("/topics", admin.CreateTopic)
		adminRoutes.PUT("/topics/:id", admin.UpdateTopic)
		adminRoutes.DELETE("/topics/:id", admin.DeleteTopic)

		// 激励语管理
		adminRoutes.GET("/quotes", admin.ListQuotes)
		adminRoutes.POST("/quotes", admin.CreateQuote)
		adminRoutes.PUT("/quotes/:id", admin.UpdateQuote)
		adminRoutes.DELETE("/quotes/:id", admin.DeleteQuote)

		// 系统监控
		adminRoutes.GET("/monitor/health", admin.HealthCheck)
		adminRoutes.GET("/monitor/stats", admin.GetStats)
	}

	return r
}
