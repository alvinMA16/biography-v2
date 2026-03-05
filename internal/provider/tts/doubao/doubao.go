package doubao

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
)

const (
	// WebSocket 服务地址
	wsURL = "wss://openspeech.bytedance.com/api/v3/sami/podcasttts"

	// Resource ID
	resourceID = "volc.service_type.10050"

	// App Key (固定值)
	appKey = "aGjiRDfUWi"
)

// Provider 豆包 TTS 提供者
type Provider struct {
	appID     string
	accessKey string
	speakers  []string
}

// New 创建豆包 TTS 提供者
func New(cfg tts.ProviderConfig) (*Provider, error) {
	if cfg.AppID == "" || cfg.AccessKey == "" {
		return nil, errors.New("doubao tts: app id and access key are required")
	}

	speakers := cfg.Speakers
	if len(speakers) == 0 {
		speakers = []string{
			"zh_male_dayixiansheng_v2_saturn_bigtts",
			"zh_female_mizaitongxue_v2_saturn_bigtts",
		}
	}

	return &Provider{
		appID:     cfg.AppID,
		accessKey: cfg.AccessKey,
		speakers:  speakers,
	}, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "doubao"
}

// Synthesize 单次语音合成
func (p *Provider) Synthesize(ctx context.Context, text string, config tts.SynthesisConfig) ([]byte, error) {
	ch, err := p.SynthesizeStream(ctx, text, config)
	if err != nil {
		return nil, err
	}

	var audio []byte
	for chunk := range ch {
		if chunk.Error != nil {
			return nil, chunk.Error
		}
		audio = append(audio, chunk.Data...)
	}

	return audio, nil
}

// SynthesizeStream 流式语音合成
func (p *Provider) SynthesizeStream(ctx context.Context, text string, config tts.SynthesisConfig) (<-chan tts.AudioChunk, error) {
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

	// 建立 WebSocket 连接
	headers := http.Header{}
	headers.Set("X-Api-App-Id", p.appID)
	headers.Set("X-Api-Access-Key", p.accessKey)
	headers.Set("X-Api-Resource-Id", resourceID)
	headers.Set("X-Api-App-Key", appKey)
	headers.Set("X-Api-Request-Id", uuid.New().String())

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	sessionID := uuid.New().String()
	audioChan := make(chan tts.AudioChunk, 100)

	// 启动会话
	session := &synthesizeSession{
		conn:       conn,
		sessionID:  sessionID,
		text:       text,
		voice:      voice,
		format:     format,
		sampleRate: sampleRate,
		speed:      config.Speed,
		audioChan:  audioChan,
	}

	go session.run(ctx)

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
	// 尝试建立连接来验证凭据
	headers := http.Header{}
	headers.Set("X-Api-App-Id", p.appID)
	headers.Set("X-Api-Access-Key", p.accessKey)
	headers.Set("X-Api-Resource-Id", resourceID)
	headers.Set("X-Api-App-Key", appKey)
	headers.Set("X-Api-Request-Id", uuid.New().String())

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return fmt.Errorf("doubao tts: health check failed: %w", err)
	}
	conn.Close()

	return nil
}

// synthesizeSession 合成会话
type synthesizeSession struct {
	conn       *websocket.Conn
	sessionID  string
	text       string
	voice      string
	format     string
	sampleRate int
	speed      int
	audioChan  chan tts.AudioChunk
	mu         sync.Mutex
}

// run 运行合成会话
func (s *synthesizeSession) run(ctx context.Context) {
	defer close(s.audioChan)
	defer s.conn.Close()

	// 发送开始请求
	if err := s.sendStart(); err != nil {
		s.audioChan <- tts.AudioChunk{Error: err}
		return
	}

	// 接收消息
	s.receiveMessages(ctx)

	// 发送结束请求
	s.sendFinish()
}

// sendStart 发送开始请求
func (s *synthesizeSession) sendStart() error {
	payload := map[string]interface{}{
		"input_id":       s.sessionID,
		"action":         3, // 对话文本直接合成
		"use_head_music": false,
		"use_tail_music": false,
		"audio_config": map[string]interface{}{
			"format":      s.format,
			"sample_rate": s.sampleRate,
			"speech_rate": s.speed,
		},
		"nlp_texts": []map[string]interface{}{
			{
				"speaker": s.voice,
				"text":    s.text,
			},
		},
	}

	frame, err := EncodeFrame(s.sessionID, eventSessionStarted, payload)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, frame)
}

// sendFinish 发送结束请求
func (s *synthesizeSession) sendFinish() error {
	frame := EncodeFinishFrame(s.sessionID)

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, frame)
}

// receiveMessages 接收消息
func (s *synthesizeSession) receiveMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 设置读取超时
		s.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		_, message, err := s.conn.ReadMessage()
		if err != nil {
			// 检查是否是正常关闭
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return
			}
			s.audioChan <- tts.AudioChunk{Error: err}
			return
		}

		frame, err := DecodeFrame(message)
		if err != nil {
			continue
		}

		switch frame.EventNumber {
		case eventSessionStarted:
			// 会话开始

		case eventPodcastRoundStart:
			// 轮次开始

		case eventPodcastRoundResp:
			// 音频数据
			if len(frame.Payload) > 0 {
				s.audioChan <- tts.AudioChunk{Data: frame.Payload}
			}

		case eventPodcastRoundEnd:
			// 轮次结束

		case eventPodcastEnd:
			// 播客结束

		case eventSessionFinished:
			// 会话结束
			s.audioChan <- tts.AudioChunk{Done: true}
			return

		case eventConnectionFinished:
			// 连接关闭
			return

		case eventUsageResponse:
			// 用量统计，忽略

		default:
			// 检查是否有错误
			if frame.MessageType == msgTypeServerError {
				resp, _ := ParsePayload(frame.Payload)
				if resp != nil && resp.StatusCode != 20000000 {
					s.audioChan <- tts.AudioChunk{
						Error: fmt.Errorf("server error: %d - %s", resp.StatusCode, resp.StatusMessage),
					}
					return
				}
			}
		}
	}
}
