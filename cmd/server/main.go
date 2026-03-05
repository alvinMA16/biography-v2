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
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/llm/dashscope"
	"github.com/peizhengma/biography-v2/internal/provider/llm/gemini"
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

	// 初始化 LLM Manager
	llmManager := initLLMProviders(cfg)

	// 初始化路由
	router := api.NewRouter(&api.RouterDeps{
		Config:     cfg,
		DB:         db,
		LLMManager: llmManager,
	})

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器（非阻塞）
	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		log.Printf("LLM providers: %v (primary: %s)", llmManager.Available(), cfg.LLMProviderDefault)
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
