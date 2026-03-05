package asr

import "context"

// Result ASR 识别结果
type Result struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	IsFinal    bool    `json:"is_final"`
}

// Provider ASR 服务提供者接口
type Provider interface {
	// Name 返回提供者名称
	Name() string

	// Transcribe 单次语音识别（一段完整音频）
	Transcribe(ctx context.Context, audio []byte, format string, sampleRate int) (*Result, error)

	// TranscribeStream 流式语音识别
	// 输入: 音频流 (PCM chunks)
	// 输出: 识别结果流 (包含中间结果和最终结果)
	TranscribeStream(ctx context.Context, audioStream <-chan []byte, format string, sampleRate int) (<-chan Result, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// ProviderConfig 提供者配置
type ProviderConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	AppKey          string
	Region          string
}
