package doubao

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
)

const (
	// V3 HTTP Chunked API 地址
	apiURL = "https://openspeech.bytedance.com/api/v3/tts/unidirectional"

	// 默认超时时间
	defaultTimeout = 30 * time.Second

	// 默认资源 ID
	defaultResourceID = "seed-tts-2.0"
)

// Provider 豆包 TTS 提供者
type Provider struct {
	appID      string
	accessKey  string
	resourceID string
	speakers   []string
	client     *http.Client
}

// New 创建豆包 TTS 提供者
func New(cfg tts.ProviderConfig) (*Provider, error) {
	if cfg.AppID == "" || cfg.AccessKey == "" {
		return nil, errors.New("doubao tts: app id and access key are required")
	}

	resourceID := cfg.ResourceID
	if resourceID == "" {
		resourceID = defaultResourceID
	}

	speakers := cfg.Speakers
	if len(speakers) == 0 {
		speakers = []string{
			"zh_male_shaonianzixin_uranus_bigtts",
			"zh_female_kefunvsheng_uranus_bigtts",
		}
	}

	return &Provider{
		appID:      cfg.AppID,
		accessKey:  cfg.AccessKey,
		resourceID: resourceID,
		speakers:   speakers,
		client: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "doubao"
}

// V3Request V3 API 请求结构
type V3Request struct {
	User      V3User      `json:"user"`
	ReqParams V3ReqParams `json:"req_params"`
}

type V3User struct {
	UID string `json:"uid"`
}

type V3ReqParams struct {
	Text        string        `json:"text"`
	Speaker     string        `json:"speaker"`
	AudioParams V3AudioParams `json:"audio_params"`
}

type V3AudioParams struct {
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	SpeechRate int    `json:"speech_rate,omitempty"`
}

// V3Response V3 API 响应结构
type V3Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

// Synthesize 单次语音合成
func (p *Provider) Synthesize(ctx context.Context, text string, config tts.SynthesisConfig) ([]byte, error) {
	// 准备配置
	voice := config.Voice
	if voice == "" {
		voice = p.speakers[0]
	}

	format := config.Format
	if format == "" {
		format = "pcm"
	}

	sampleRate := config.SampleRate
	if sampleRate == 0 {
		sampleRate = 24000
	}

	// 构建请求
	v3Req := V3Request{
		User: V3User{
			UID: "biography-user",
		},
		ReqParams: V3ReqParams{
			Text:    text,
			Speaker: voice,
			AudioParams: V3AudioParams{
				Format:     format,
				SampleRate: sampleRate,
				SpeechRate: config.Speed,
			},
		},
	}

	reqBody, err := json.Marshal(v3Req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 V3 API Headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-App-Id", p.appID)
	req.Header.Set("X-Api-Access-Key", p.accessKey)
	req.Header.Set("X-Api-Resource-Id", p.resourceID)
	req.Header.Set("X-Api-Request-Id", uuid.New().String())

	// 发送请求
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts request failed with status: %d", resp.StatusCode)
	}

	// 读取流式响应，拼接所有音频数据
	var audioData []byte
	scanner := bufio.NewScanner(resp.Body)
	// 增大 buffer 以处理大的音频数据块
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var v3Resp V3Response
		if err := json.Unmarshal(line, &v3Resp); err != nil {
			// 跳过无法解析的行
			continue
		}

		// code=0 表示音频数据，code=20000000 表示结束
		if v3Resp.Code == 0 && v3Resp.Data != "" {
			chunk, err := base64.StdEncoding.DecodeString(v3Resp.Data)
			if err != nil {
				continue
			}
			audioData = append(audioData, chunk...)
		} else if v3Resp.Code == 20000000 {
			// 合成完成
			break
		} else if v3Resp.Code != 0 && v3Resp.Code != 20000000 {
			// 错误响应
			return nil, fmt.Errorf("tts failed: code=%d, message=%s", v3Resp.Code, v3Resp.Message)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if len(audioData) == 0 {
		return nil, errors.New("no audio data received")
	}

	return audioData, nil
}

// SynthesizeStream 流式语音合成
func (p *Provider) SynthesizeStream(ctx context.Context, text string, config tts.SynthesisConfig) (<-chan tts.AudioChunk, error) {
	audioChan := make(chan tts.AudioChunk, 10)

	go func() {
		defer close(audioChan)

		// 准备配置
		voice := config.Voice
		if voice == "" {
			voice = p.speakers[0]
		}

		format := config.Format
		if format == "" {
			format = "pcm"
		}

		sampleRate := config.SampleRate
		if sampleRate == 0 {
			sampleRate = 24000
		}

		// 构建请求
		v3Req := V3Request{
			User: V3User{
				UID: "biography-user",
			},
			ReqParams: V3ReqParams{
				Text:    text,
				Speaker: voice,
				AudioParams: V3AudioParams{
					Format:     format,
					SampleRate: sampleRate,
					SpeechRate: config.Speed,
				},
			},
		}

		reqBody, err := json.Marshal(v3Req)
		if err != nil {
			audioChan <- tts.AudioChunk{Error: fmt.Errorf("failed to marshal request: %w", err)}
			return
		}

		// 创建 HTTP 请求
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(reqBody))
		if err != nil {
			audioChan <- tts.AudioChunk{Error: fmt.Errorf("failed to create request: %w", err)}
			return
		}

		// 设置 V3 API Headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-App-Id", p.appID)
		req.Header.Set("X-Api-Access-Key", p.accessKey)
		req.Header.Set("X-Api-Resource-Id", p.resourceID)
		req.Header.Set("X-Api-Request-Id", uuid.New().String())

		// 发送请求
		resp, err := p.client.Do(req)
		if err != nil {
			audioChan <- tts.AudioChunk{Error: fmt.Errorf("failed to send request: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			audioChan <- tts.AudioChunk{Error: fmt.Errorf("tts request failed with status: %d", resp.StatusCode)}
			return
		}

		// 流式读取响应
		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var v3Resp V3Response
			if err := json.Unmarshal(line, &v3Resp); err != nil {
				continue
			}

			if v3Resp.Code == 0 && v3Resp.Data != "" {
				chunk, err := base64.StdEncoding.DecodeString(v3Resp.Data)
				if err != nil {
					continue
				}
				audioChan <- tts.AudioChunk{Data: chunk}
			} else if v3Resp.Code == 20000000 {
				audioChan <- tts.AudioChunk{Done: true}
				return
			} else if v3Resp.Code != 0 && v3Resp.Code != 20000000 {
				audioChan <- tts.AudioChunk{Error: fmt.Errorf("tts failed: code=%d, message=%s", v3Resp.Code, v3Resp.Message)}
				return
			}
		}

		if err := scanner.Err(); err != nil {
			audioChan <- tts.AudioChunk{Error: fmt.Errorf("failed to read response: %w", err)}
		}
	}()

	return audioChan, nil
}

// ListVoices 获取可用音色列表
func (p *Provider) ListVoices(ctx context.Context) ([]tts.Voice, error) {
	voices := []tts.Voice{
		{
			ID:          "zh_male_shaonianzixin_uranus_bigtts",
			Name:        "少年梓辛 / Brayan 2.0",
			Gender:      "male",
			Description: "通用场景男声",
		},
		{
			ID:          "zh_female_kefunvsheng_uranus_bigtts",
			Name:        "暖阳女声 2.0",
			Gender:      "female",
			Description: "通用场景女声",
		},
		{
			ID:          "zh_male_liufei_uranus_bigtts",
			Name:        "刘飞 2.0",
			Gender:      "male",
			Description: "通用场景男声",
		},
		{
			ID:          "zh_female_shuangkuaisisi_moon_bigtts",
			Name:        "爽快思思",
			Gender:      "female",
			Description: "通用场景女声",
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
