package config

import (
	"fmt"
	"os"
	"strings"
)

// Config 应用配置
type Config struct {
	// 服务器
	Port string
	Env  string // development, production

	// 数据库
	DatabaseURL string

	// JWT
	JWTSecret     string
	JWTExpireDays int

	// Admin
	AdminAPIKey string

	// LLM Providers
	LLMProviderDefault string // gemini, dashscope, openai

	// Gemini
	GeminiAPIKey    string
	GeminiModel     string
	GeminiModelFast string
	GeminiProxy     string

	// DashScope (阿里通义)
	DashScopeAPIKey    string
	DashScopeBaseURL   string
	DashScopeModel     string
	DashScopeModelFast string

	// ASR (阿里语音识别)
	AliyunAccessKeyID     string
	AliyunAccessKeySecret string
	AliyunASRAppKey       string

	// TTS (豆包 V3 API, seed-tts-2.0)
	DoubaoAppID      string
	DoubaoAccessKey  string
	DoubaoResourceID string
	DoubaoSpeakers   []string
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	cfg := &Config{
		Port: getEnv("PORT", "8000"),
		Env:  getEnv("ENV", "development"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://biography:biography@localhost:5432/biography?sslmode=disable"),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpireDays: getEnvInt("JWT_EXPIRE_DAYS", 30),

		AdminAPIKey: getEnv("ADMIN_API_KEY", ""),

		LLMProviderDefault: "gemini",

		GeminiAPIKey:    getEnv("GEMINI_API_KEY", ""),
		GeminiModel:     "gemini-3.1-flash-image-preview",
		GeminiModelFast: "gemini-2.0-flash-lite",
		GeminiProxy:     getEnv("GEMINI_PROXY", ""),

		DashScopeAPIKey:    getEnv("DASHSCOPE_API_KEY", ""),
		DashScopeBaseURL:   getEnv("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		DashScopeModel:     "qwen3.5-plus",
		DashScopeModelFast: "qwen-turbo",

		AliyunAccessKeyID:     getEnv("ALIYUN_ACCESS_KEY_ID", ""),
		AliyunAccessKeySecret: getEnv("ALIYUN_ACCESS_KEY_SECRET", ""),
		AliyunASRAppKey:       getEnv("ALIYUN_ASR_APP_KEY", ""),

		DoubaoAppID:      getEnv("DOUBAO_APP_ID", ""),
		DoubaoAccessKey:  getEnv("DOUBAO_ACCESS_KEY", ""),
		DoubaoResourceID: getEnv("DOUBAO_RESOURCE_ID", "seed-tts-2.0"),
		DoubaoSpeakers:   getEnvList("DOUBAO_SPEAKERS", []string{"zh_male_shaonianzixin_uranus_bigtts", "zh_female_kefunvsheng_uranus_bigtts"}),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvList(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
