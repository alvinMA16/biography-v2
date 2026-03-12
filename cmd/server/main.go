package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/peizhengma/biography-v2/internal/api"
	"github.com/peizhengma/biography-v2/internal/config"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/asr/aliyun"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/llm/dashscope"
	"github.com/peizhengma/biography-v2/internal/provider/llm/gemini"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
	"github.com/peizhengma/biography-v2/internal/provider/tts/doubao"
	auditRepo "github.com/peizhengma/biography-v2/internal/repository/audit"
	convRepo "github.com/peizhengma/biography-v2/internal/repository/conversation"
	eraRepo "github.com/peizhengma/biography-v2/internal/repository/era"
	memoirRepo "github.com/peizhengma/biography-v2/internal/repository/memoir"
	presetRepo "github.com/peizhengma/biography-v2/internal/repository/preset"
	quoteRepo "github.com/peizhengma/biography-v2/internal/repository/quote"
	topicRepo "github.com/peizhengma/biography-v2/internal/repository/topic"
	turnTraceRepo "github.com/peizhengma/biography-v2/internal/repository/turntrace"
	userRepo "github.com/peizhengma/biography-v2/internal/repository/user"
	welcomeRepo "github.com/peizhengma/biography-v2/internal/repository/welcome"
	auditService "github.com/peizhengma/biography-v2/internal/service/audit"
	convService "github.com/peizhengma/biography-v2/internal/service/conversation"
	eraService "github.com/peizhengma/biography-v2/internal/service/era"
	flowService "github.com/peizhengma/biography-v2/internal/service/flow"
	llmService "github.com/peizhengma/biography-v2/internal/service/llm"
	memoirService "github.com/peizhengma/biography-v2/internal/service/memoir"
	presetService "github.com/peizhengma/biography-v2/internal/service/preset"
	quoteService "github.com/peizhengma/biography-v2/internal/service/quote"
	topicService "github.com/peizhengma/biography-v2/internal/service/topic"
	turnTraceService "github.com/peizhengma/biography-v2/internal/service/turntrace"
	userService "github.com/peizhengma/biography-v2/internal/service/user"
	welcomeService "github.com/peizhengma/biography-v2/internal/service/welcome"
	"github.com/peizhengma/biography-v2/internal/storage/postgres"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	db, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 初始化 Providers
	llmManager := initLLMProviders(cfg)
	asrProvider := initASRProvider(cfg)
	ttsProvider := initTTSProvider(cfg)

	// 初始化 Repository 和 Service
	userRepository := userRepo.New(db.Pool())
	userSvc := userService.New(userRepository, userService.Config{
		JWTSecret:     cfg.JWTSecret,
		JWTExpireDays: cfg.JWTExpireDays,
	})

	convRepository := convRepo.New(db.Pool())
	convSvc := convService.New(convRepository)

	memoirRepository := memoirRepo.New(db.Pool())
	memoirSvc := memoirService.New(memoirRepository)

	topicRepository := topicRepo.New(db.Pool())
	topicSvc := topicService.New(topicRepository)

	turnTraceRepository := turnTraceRepo.New(db.Pool())
	turnTraceSvc := turnTraceService.New(turnTraceRepository)

	quoteRepository := quoteRepo.New(db.Pool())
	quoteSvc := quoteService.New(quoteRepository)

	eraRepository := eraRepo.New(db.Pool())
	eraSvc := eraService.New(eraRepository)

	presetRepository := presetRepo.New(db.Pool())
	presetSvc := presetService.New(presetRepository)

	welcomeRepository := welcomeRepo.New(db.Pool())
	welcomeSvc := welcomeService.New(welcomeRepository)

	auditRepository := auditRepo.New(db.Pool())
	auditSvc := auditService.New(auditRepository)

	// 初始化 LLM Service
	llmSvc := llmService.New(llmManager)

	// 初始化 Flow Service
	flowSvc := flowService.New(userSvc, convSvc, memoirSvc, topicSvc, llmSvc)

	// 初始化路由
	router := api.NewRouter(&api.RouterDeps{
		Config:              cfg,
		DB:                  db,
		LLMManager:          llmManager,
		ASRProvider:         asrProvider,
		TTSProvider:         ttsProvider,
		UserService:         userSvc,
		ConversationService: convSvc,
		MemoirService:       memoirSvc,
		TopicService:        topicSvc,
		QuoteService:        quoteSvc,
		LLMService:          llmSvc,
		EraService:          eraSvc,
		PresetService:       presetSvc,
		WelcomeService:      welcomeSvc,
		AuditService:        auditSvc,
		FlowService:         flowSvc,
		TurnTraceService:    turnTraceSvc,
	})

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器（非阻塞）
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		log.Printf("LLM providers: %v (primary: %s)", llmManager.Available(), cfg.LLMProviderDefault)
		if asrProvider != nil {
			log.Printf("ASR provider: %s", asrProvider.Name())
		}
		if ttsProvider != nil {
			log.Printf("TTS provider: %s", ttsProvider.Name())
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// initLLMProviders 初始化 LLM 提供者
func initLLMProviders(cfg *config.Config) *llm.Manager {
	manager := llm.NewManager(cfg.LLMProviderDefault)

	// 初始化 Gemini
	if cfg.GeminiAPIKey != "" {
		geminiProvider, err := gemini.New(llm.ProviderConfig{
			APIKey:    cfg.GeminiAPIKey,
			Model:     cfg.GeminiModel,
			ModelFast: cfg.GeminiModelFast,
			Proxy:     cfg.GeminiProxy,
			Timeout:   60,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize Gemini provider: %v", err)
		} else {
			manager.Register(geminiProvider)
			log.Printf("Gemini provider initialized (model: %s)", cfg.GeminiModel)
		}

		if cfg.GeminiRealtimeHedgeModel1 != "" {
			hedgeProvider, err := gemini.New(llm.ProviderConfig{
				APIKey:    cfg.GeminiAPIKey,
				Model:     cfg.GeminiRealtimeHedgeModel1,
				ModelFast: cfg.GeminiModelFast,
				Proxy:     cfg.GeminiProxy,
				Timeout:   60,
			})
			if err != nil {
				log.Printf("Warning: Failed to initialize Gemini realtime hedge provider 1: %v", err)
			} else {
				manager.Register(llm.WithAlias(llm.ProviderGeminiRealtimePreview, hedgeProvider))
				log.Printf("Gemini realtime hedge provider initialized (name: %s model: %s)", llm.ProviderGeminiRealtimePreview, cfg.GeminiRealtimeHedgeModel1)
			}
		}

		if cfg.GeminiRealtimeHedgeModel2 != "" {
			hedgeProvider, err := gemini.New(llm.ProviderConfig{
				APIKey:    cfg.GeminiAPIKey,
				Model:     cfg.GeminiRealtimeHedgeModel2,
				ModelFast: cfg.GeminiModelFast,
				Proxy:     cfg.GeminiProxy,
				Timeout:   60,
			})
			if err != nil {
				log.Printf("Warning: Failed to initialize Gemini realtime hedge provider 2: %v", err)
			} else {
				manager.Register(llm.WithAlias(llm.ProviderGeminiRealtimeFast, hedgeProvider))
				log.Printf("Gemini realtime hedge provider initialized (name: %s model: %s)", llm.ProviderGeminiRealtimeFast, cfg.GeminiRealtimeHedgeModel2)
			}
		}
	}

	// 初始化 DashScope
	if cfg.DashScopeAPIKey != "" {
		dashscopeProvider, err := dashscope.New(llm.ProviderConfig{
			APIKey:    cfg.DashScopeAPIKey,
			BaseURL:   cfg.DashScopeBaseURL,
			Model:     cfg.DashScopeModel,
			ModelFast: cfg.DashScopeModelFast,
			Timeout:   60,
		})
		if err != nil {
			log.Printf("Warning: Failed to initialize DashScope provider: %v", err)
		} else {
			manager.Register(dashscopeProvider)
			log.Printf("DashScope provider initialized (model: %s)", cfg.DashScopeModel)
		}
	}

	// 检查是否有可用的 Provider
	if len(manager.Available()) == 0 {
		log.Println("Warning: No LLM providers configured")
	}

	return manager
}

// initASRProvider 初始化 ASR 提供者
func initASRProvider(cfg *config.Config) asr.Provider {
	if cfg.AliyunAccessKeyID == "" || cfg.AliyunAccessKeySecret == "" || cfg.AliyunASRAppKey == "" {
		log.Println("Warning: Aliyun ASR not configured (require ALIYUN_ACCESS_KEY_ID, ALIYUN_ACCESS_KEY_SECRET, ALIYUN_ASR_APP_KEY)")
		return nil
	}

	provider, err := aliyun.New(asr.ProviderConfig{
		AccessKeyID:     cfg.AliyunAccessKeyID,
		AccessKeySecret: cfg.AliyunAccessKeySecret,
		AppKey:          cfg.AliyunASRAppKey,
		Region:          "cn-shanghai",
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize Aliyun ASR provider: %v", err)
		return nil
	}

	log.Println("Aliyun ASR provider initialized")
	return provider
}

// initTTSProvider 初始化 TTS 提供者
func initTTSProvider(cfg *config.Config) tts.Provider {
	if cfg.DoubaoAppID == "" || cfg.DoubaoAccessKey == "" {
		log.Println("Warning: Doubao TTS not configured")
		return nil
	}

	provider, err := doubao.New(tts.ProviderConfig{
		AppID:      cfg.DoubaoAppID,
		AccessKey:  cfg.DoubaoAccessKey,
		ResourceID: cfg.DoubaoResourceID,
		Speakers:   cfg.DoubaoSpeakers,
	})
	if err != nil {
		log.Printf("Warning: Failed to initialize Doubao TTS provider: %v", err)
		return nil
	}

	log.Println("Doubao TTS provider initialized")
	return provider
}
