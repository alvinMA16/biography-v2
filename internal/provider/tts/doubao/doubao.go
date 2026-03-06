package doubao

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
)

const (
	// HTTP API 地址
	apiURL = "https://openspeech.bytedance.com/api/v1/tts"

	// 默认超时时间
	defaultTimeout = 30 * time.Second
)

// Provider 豆包 TTS 提供者
type Provider struct {
	appID    string
	token    string
	cluster  string
	speakers []string
	client   *http.Client
}

// New 创建豆包 TTS 提供者
func New(cfg tts.ProviderConfig) (*Provider, error) {
	if cfg.AppID == "" || cfg.Token == "" {
		return nil, errors.New("doubao tts: app id and token are required")
	}

	cluster := cfg.Cluster
	if cluster == "" {
		cluster = "volcano_tts"
	}

	speakers := cfg.Speakers
	if len(speakers) == 0 {
		speakers = []string{
			"zh_male_dayixiansheng_v2_saturn_bigtts",
			"zh_female_mizaitongxue_v2_saturn_bigtts",
		}
	}

	return &Provider{
		appID:    cfg.AppID,
		token:    cfg.Token,
		cluster:  cluster,
		speakers: speakers,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "doubao"
}

// TTSRequest TTS 请求结构
type TTSRequest struct {
	App     AppConfig     `json:"app"`
	User    UserConfig    `json:"user"`
	Audio   AudioConfig   `json:"audio"`
	Request RequestConfig `json:"request"`
}

type AppConfig struct {
	AppID   string `json:"appid"`
	Token   string `json:"token"`
	Cluster string `json:"cluster"`
}

type UserConfig struct {
	UID string `json:"uid"`
}

type AudioConfig struct {
	VoiceType   string  `json:"voice_type"`
	Encoding    string  `json:"encoding"`
	SpeedRatio  float64 `json:"speed_ratio"`
	VolumeRatio float64 `json:"volume_ratio"`
	PitchRatio  float64 `json:"pitch_ratio"`
}

type RequestConfig struct {
	ReqID     string `json:"reqid"`
	Text      string `json:"text"`
	TextType  string `json:"text_type"`
	Operation string `json:"operation"`
}

// TTSResponse TTS 响应结构
type TTSResponse struct {
	ReqID     string `json:"reqid"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Operation string `json:"operation"`
	Sequence  int    `json:"sequence"`
	Data      string `json:"data"`
}

// Synthesize 单次语音合成
func (p *Provider) Synthesize(ctx context.Context, text string, config tts.SynthesisConfig) ([]byte, error) {
	// 准备配置
	voice := config.Voice
	if voice == "" {
		voice = p.speakers[0]
	}

	encoding := config.Format
	if encoding == "" {
		encoding = "pcm"
	}
	// 转换格式名称
	if encoding == "pcm" {
		encoding = "pcm"
	}

	// 计算语速比例 (speed: -50~100 -> speedRatio: 0.5~2.0)
	speedRatio := 1.0
	if config.Speed != 0 {
		speedRatio = 1.0 + float64(config.Speed)/100.0
		if speedRatio < 0.5 {
			speedRatio = 0.5
		}
		if speedRatio > 2.0 {
			speedRatio = 2.0
		}
	}

	// 构建请求
	reqID := uuid.New().String()
	ttsReq := TTSRequest{
		App: AppConfig{
			AppID:   p.appID,
			Token:   "access_token", // 固定值，实际认证用 Header
			Cluster: p.cluster,
		},
		User: UserConfig{
			UID: "biography-user",
		},
		Audio: AudioConfig{
			VoiceType:   voice,
			Encoding:    encoding,
			SpeedRatio:  speedRatio,
			VolumeRatio: 1.0,
			PitchRatio:  1.0,
		},
		Request: RequestConfig{
			ReqID:     reqID,
			Text:      text,
			TextType:  "plain",
			Operation: "query",
		},
	}

	reqBody, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer;%s", p.token))

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应
	var ttsResp TTSResponse
	if err := json.Unmarshal(respBody, &ttsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 检查响应码 (3000 表示成功)
	if ttsResp.Code != 3000 {
		return nil, fmt.Errorf("tts failed: code=%d, message=%s", ttsResp.Code, ttsResp.Message)
	}

	// Base64 解码音频数据
	audio, err := base64.StdEncoding.DecodeString(ttsResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode audio data: %w", err)
	}

	return audio, nil
}

// SynthesizeStream 流式语音合成（HTTP 接口不支持真正的流式，返回完整音频作为单个 chunk）
func (p *Provider) SynthesizeStream(ctx context.Context, text string, config tts.SynthesisConfig) (<-chan tts.AudioChunk, error) {
	audioChan := make(chan tts.AudioChunk, 1)

	go func() {
		defer close(audioChan)

		audio, err := p.Synthesize(ctx, text, config)
		if err != nil {
			audioChan <- tts.AudioChunk{Error: err}
			return
		}

		audioChan <- tts.AudioChunk{Data: audio, Done: true}
	}()

	return audioChan, nil
}

// ListVoices 获取可用音色列表
func (p *Provider) ListVoices(ctx context.Context) ([]tts.Voice, error) {
	voices := []tts.Voice{
		{
			ID:          "zh_male_dayixiansheng_v2_saturn_bigtts",
			Name:        "大义先生",
			Gender:      "male",
			Description: "黑猫侦探社系列男声",
		},
		{
			ID:          "zh_female_mizaitongxue_v2_saturn_bigtts",
			Name:        "咪仔同学",
			Gender:      "female",
			Description: "黑猫侦探社系列女声",
		},
		{
			ID:          "zh_male_liufei_v2_saturn_bigtts",
			Name:        "刘飞",
			Gender:      "male",
			Description: "刘飞和潇磊系列男声",
		},
		{
			ID:          "zh_male_xiaolei_v2_saturn_bigtts",
			Name:        "潇磊",
			Gender:      "male",
			Description: "刘飞和潇磊系列男声",
		},
	}

	return voices, nil
}

// HealthCheck 健康检查
func (p *Provider) HealthCheck(ctx context.Context) error {
	// 尝试合成一小段文字来验证凭据
	_, err := p.Synthesize(ctx, "测试", tts.SynthesisConfig{
		Voice:  p.speakers[0],
		Format: "pcm",
	})
	if err != nil {
		return fmt.Errorf("doubao tts: health check failed: %w", err)
	}

	return nil
}
