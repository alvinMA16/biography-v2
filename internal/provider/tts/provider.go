package tts

import "context"

// AudioChunk 音频片段
type AudioChunk struct {
	Data  []byte
	Done  bool
	Error error
}

// SynthesisConfig 合成配置
type SynthesisConfig struct {
	Voice      string // 音色
	Format     string // 音频格式: pcm, mp3, ogg_opus
	SampleRate int    // 采样率: 16000, 24000, 48000
	Speed      int    // 语速: -50 ~ 100
}

// Provider TTS 服务提供者接口
type Provider interface {
	// Name 返回提供者名称
	Name() string

	// Synthesize 单次语音合成（返回完整音频）
	Synthesize(ctx context.Context, text string, config SynthesisConfig) ([]byte, error)

	// SynthesizeStream 流式语音合成
	// 输入: 要合成的文本
	// 输出: 音频流
	SynthesizeStream(ctx context.Context, text string, config SynthesisConfig) (<-chan AudioChunk, error)

	// ListVoices 获取可用音色列表
	ListVoices(ctx context.Context) ([]Voice, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// Voice 音色信息
type Voice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Gender      string `json:"gender"` // male, female
	Description string `json:"description"`
}

// ProviderConfig 提供者配置
type ProviderConfig struct {
	AppID     string
	AccessKey string
	Speakers  []string
}
