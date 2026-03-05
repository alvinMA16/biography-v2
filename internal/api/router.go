package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/peizhengma/biography-v2/internal/api/admin"
	"github.com/peizhengma/biography-v2/internal/api/middleware"
	"github.com/peizhengma/biography-v2/internal/api/realtime"
	"github.com/peizhengma/biography-v2/internal/api/user"
	"github.com/peizhengma/biography-v2/internal/config"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	eraService "github.com/peizhengma/biography-v2/internal/service/era"
	llmService "github.com/peizhengma/biography-v2/internal/service/llm"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	presetService "github.com/peizhengma/biography-v2/internal/service/preset"
	quoteService "github.com/peizhengma/biography-v2/internal/service/quote"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
	"github.com/peizhengma/biography-v2/internal/storage/postgres"
)

// RouterDeps 路由依赖
type RouterDeps struct {
	Config              *config.Config
	DB                  *postgres.DB
	LLMManager          *llm.Manager
	ASRProvider         asr.Provider
	TTSProvider         tts.Provider
	UserService         *userService.Service
	ConversationService *convService.Service
	MemoirService       *memoirService.Service
	TopicService        *topicService.Service
	QuoteService        *quoteService.Service
	LLMService          *llmService.Service
	EraService          *eraService.Service
	PresetService       *presetService.Service
}

// NewRouter 创建路由
func NewRouter(deps *RouterDeps) http.Handler {
	if deps.Config.Env == "production" {
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

	// 创建 User Handler
	userHandler := user.NewHandler(deps.UserService, deps.ConversationService, deps.MemoirService, deps.TopicService)

	// API 路由组
	api := r.Group("/api")

	// 公开路由
	api.POST("/auth/register", userHandler.Register)
	api.POST("/auth/login", userHandler.Login)

	// 用户端路由（需要 JWT 认证）
	userRoutes := api.Group("")
	userRoutes.Use(middleware.JWTAuth(deps.Config.JWTSecret))
	{
		// 用户信息
		userRoutes.GET("/user/profile", userHandler.GetProfile)
		userRoutes.PUT("/user/profile", userHandler.UpdateProfile)
		userRoutes.PUT("/user/password", userHandler.ChangePassword)

		// 对话
		userRoutes.GET("/conversations", userHandler.ListConversations)
		userRoutes.POST("/conversations", userHandler.CreateConversation)
		userRoutes.GET("/conversations/:id", userHandler.GetConversation)
		userRoutes.GET("/conversations/:id/messages", userHandler.GetMessages)

		// 回忆录
		userRoutes.GET("/memoirs", userHandler.ListMemoirs)
		userRoutes.GET("/memoirs/:id", userHandler.GetMemoir)

		// 话题
		userRoutes.GET("/topics", userHandler.GetTopicOptions)
	}

	// 创建 Realtime Handler
	realtimeHandler := realtime.NewHandler(
		deps.Config.JWTSecret,
		deps.UserService,
		deps.TopicService,
		deps.ConversationService,
		deps.MemoirService,
		deps.LLMService,
		deps.ASRProvider,
		deps.LLMManager,
		deps.TTSProvider,
	)

	// WebSocket 实时对话（需要 JWT 认证，通过 query param）
	api.GET("/realtime/dialog", realtimeHandler.HandleDialog)

	// 创建 Admin Handler
	adminHandler := admin.NewHandler(
		deps.LLMManager,
		deps.ASRProvider,
		deps.TTSProvider,
		deps.UserService,
		deps.ConversationService,
		deps.MemoirService,
		deps.TopicService,
		deps.QuoteService,
		deps.LLMService,
		deps.EraService,
		deps.PresetService,
	)

	// 管理端路由（需要 Admin API Key）
	adminRoutes := api.Group("/admin")
	adminRoutes.Use(middleware.AdminAuth(deps.Config.AdminAPIKey))
	{
		// 用户管理
		adminRoutes.GET("/users", adminHandler.ListUsers)
		adminRoutes.GET("/users/:id", adminHandler.GetUser)
		adminRoutes.PUT("/users/:id", adminHandler.UpdateUser)
		adminRoutes.DELETE("/users/:id", adminHandler.DeleteUser)

		// 对话管理
		adminRoutes.GET("/conversations", adminHandler.ListConversations)
		adminRoutes.GET("/conversations/:id", adminHandler.GetConversation)

		// 回忆录管理
		adminRoutes.GET("/memoirs", adminHandler.ListMemoirs)
		adminRoutes.PUT("/memoirs/:id", adminHandler.UpdateMemoir)
		adminRoutes.DELETE("/memoirs/:id", adminHandler.DeleteMemoir)
		adminRoutes.POST("/memoirs/:id/regenerate", adminHandler.RegenerateMemoir)

		// 话题管理
		adminRoutes.GET("/topics", adminHandler.ListTopics)
		adminRoutes.POST("/topics", adminHandler.CreateTopic)
		adminRoutes.PUT("/topics/:id", adminHandler.UpdateTopic)
		adminRoutes.DELETE("/topics/:id", adminHandler.DeleteTopic)

		// 激励语管理
		adminRoutes.GET("/quotes", adminHandler.ListQuotes)
		adminRoutes.POST("/quotes", adminHandler.CreateQuote)
		adminRoutes.PUT("/quotes/:id", adminHandler.UpdateQuote)
		adminRoutes.DELETE("/quotes/:id", adminHandler.DeleteQuote)

		// LLM Provider 管理
		adminRoutes.GET("/llm/providers", adminHandler.GetLLMProviders)
		adminRoutes.PUT("/llm/providers/primary", adminHandler.SetLLMProvider)
		adminRoutes.POST("/llm/providers/:provider/test", adminHandler.TestLLMProvider)

		// TTS Provider 管理
		adminRoutes.GET("/tts/voices", adminHandler.GetTTSVoices)
		adminRoutes.POST("/tts/test", adminHandler.TestTTS)

		// 时代记忆预设管理
		adminRoutes.GET("/era-memories", adminHandler.ListEraMemories)
		adminRoutes.POST("/era-memories", adminHandler.CreateEraMemory)
		adminRoutes.PUT("/era-memories/:id", adminHandler.UpdateEraMemory)
		adminRoutes.DELETE("/era-memories/:id", adminHandler.DeleteEraMemory)

		// 预设话题管理
		adminRoutes.GET("/preset-topics", adminHandler.ListPresetTopics)
		adminRoutes.POST("/preset-topics", adminHandler.CreatePresetTopic)
		adminRoutes.PUT("/preset-topics/:id", adminHandler.UpdatePresetTopic)
		adminRoutes.DELETE("/preset-topics/:id", adminHandler.DeletePresetTopic)

		// 系统监控
		adminRoutes.GET("/monitor/health", adminHandler.HealthCheck)
		adminRoutes.GET("/monitor/stats", adminHandler.GetStats)
	}

	return r
}
