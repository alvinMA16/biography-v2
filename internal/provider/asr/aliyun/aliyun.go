package aliyun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
)

const (
	// WebSocket 服务地址
	wsURLTemplate = "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1?token=%s"

	// 命名空间
	namespaceTranscriber = "SpeechTranscriber"

	// 事件名称
	eventStartTranscription = "StartTranscription"
	eventStopTranscription  = "StopTranscription"
	eventSentenceBegin      = "SentenceBegin"
	eventSentenceEnd        = "SentenceEnd"
	eventResultChanged      = "TranscriptionResultChanged"
	eventCompleted          = "TranscriptionCompleted"
	eventFailed             = "TaskFailed"
)

// Provider 阿里云 ASR 提供者
type Provider struct {
	appKey       string
	tokenManager *TokenManager
	config       ProviderConfig
}

// ProviderConfig 扩展配置
type ProviderConfig struct {
	EnableIntermediateResult bool
	EnablePunctuation        bool
	EnableITN                bool // Inverse Text Normalization
	MaxSentenceSilence       int  // ms, 200-6000
	EnableWords              bool
	Disfluency               bool // 过滤语气词
}

// New 创建阿里云 ASR 提供者
func New(cfg asr.ProviderConfig) (*Provider, error) {
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, errors.New("aliyun asr: access key is required")
	}
	if cfg.AppKey == "" {
		return nil, errors.New("aliyun asr: app key is required")
	}

	return &Provider{
		appKey:       cfg.AppKey,
		tokenManager: NewTokenManager(cfg.AccessKeyID, cfg.AccessKeySecret),
		config: ProviderConfig{
			EnableIntermediateResult: true,
			EnablePunctuation:        true,
			EnableITN:                true,
			MaxSentenceSilence:       800,
			Disfluency:               true,
		},
	}, nil
}

// Name 返回提供者名称
func (p *Provider) Name() string {
	return "aliyun"
}

// UpstreamEndpoint 返回上游服务地址（供监控展示）
func (p *Provider) UpstreamEndpoint() string {
	return "wss://nls-gateway-cn-shanghai.aliyuncs.com/ws/v1 (token: https://nls-meta.cn-shanghai.aliyuncs.com/)"
}

// Transcribe 单次语音识别
func (p *Provider) Transcribe(ctx context.Context, audio []byte, format string, sampleRate int) (*asr.Result, error) {
	// 创建音频通道
	audioChan := make(chan []byte, 1)
	audioChan <- audio
	close(audioChan)

	// 使用流式接口
	resultChan, err := p.TranscribeStream(ctx, audioChan, format, sampleRate)
	if err != nil {
		return nil, err
	}

	// 收集所有结果
	var finalResult *asr.Result
	for result := range resultChan {
		if result.IsFinal {
			finalResult = &result
		}
	}

	if finalResult == nil {
		return nil, errors.New("no transcription result")
	}

	return finalResult, nil
}

// TranscribeStream 流式语音识别
func (p *Provider) TranscribeStream(ctx context.Context, audioStream <-chan []byte, format string, sampleRate int) (<-chan asr.Result, error) {
	// 获取 Token
	token, err := p.tokenManager.GetToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	// 建立 WebSocket 连接
	wsURL := fmt.Sprintf(wsURLTemplate, token)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	taskID := uuid.New().String()
	resultChan := make(chan asr.Result, 100)

	// 启动会话
	session := &transcribeSession{
		conn:        conn,
		taskID:      taskID,
		appKey:      p.appKey,
		format:      format,
		sampleRate:  sampleRate,
		config:      p.config,
		audioStream: audioStream,
		resultChan:  resultChan,
	}

	go session.run(ctx)

	return resultChan, nil
}

// HealthCheck 健康检查
func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.tokenManager.GetToken()
	if err != nil {
		return fmt.Errorf("aliyun asr: health check failed: %w", err)
	}
	return nil
}

// transcribeSession 转写会话
type transcribeSession struct {
	conn        *websocket.Conn
	taskID      string
	appKey      string
	format      string
	sampleRate  int
	config      ProviderConfig
	audioStream <-chan []byte
	resultChan  chan asr.Result
	mu          sync.Mutex
}

// run 运行转写会话
func (s *transcribeSession) run(ctx context.Context) {
	defer close(s.resultChan)
	defer s.conn.Close()

	// 发送开始命令
	if err := s.sendStart(); err != nil {
		s.resultChan <- asr.Result{IsFinal: true}
		return
	}

	// 启动消息接收
	done := make(chan struct{})
	go s.receiveMessages(done)

	// 发送音频数据
	s.sendAudio(ctx)

	// 发送停止命令
	s.sendStop()

	// 等待接收完成
	select {
	case <-done:
	case <-time.After(10 * time.Second):
	}
}

// sendStart 发送开始识别命令
func (s *transcribeSession) sendStart() error {
	msg := map[string]interface{}{
		"header": map[string]interface{}{
			"message_id": uuid.New().String(),
			"task_id":    s.taskID,
			"namespace":  namespaceTranscriber,
			"name":       eventStartTranscription,
			"appkey":     s.appKey,
		},
		"payload": map[string]interface{}{
			"format":                            s.format,
			"sample_rate":                       s.sampleRate,
			"enable_intermediate_result":        s.config.EnableIntermediateResult,
			"enable_punctuation_prediction":     s.config.EnablePunctuation,
			"enable_inverse_text_normalization": s.config.EnableITN,
			"max_sentence_silence":              s.config.MaxSentenceSilence,
			"enable_words":                      s.config.EnableWords,
			"disfluency":                        s.config.Disfluency,
		},
	}

	return s.conn.WriteJSON(msg)
}

// sendStop 发送停止识别命令
func (s *transcribeSession) sendStop() error {
	msg := map[string]interface{}{
		"header": map[string]interface{}{
			"message_id": uuid.New().String(),
			"task_id":    s.taskID,
			"namespace":  namespaceTranscriber,
			"name":       eventStopTranscription,
			"appkey":     s.appKey,
		},
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(msg)
}

// sendAudio 发送音频数据
func (s *transcribeSession) sendAudio(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case audio, ok := <-s.audioStream:
			if !ok {
				return
			}
			s.mu.Lock()
			err := s.conn.WriteMessage(websocket.BinaryMessage, audio)
			s.mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// receiveMessages 接收消息
func (s *transcribeSession) receiveMessages(done chan struct{}) {
	defer close(done)

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			return
		}

		var resp response
		if err := json.Unmarshal(message, &resp); err != nil {
			continue
		}

		switch resp.Header.Name {
		case eventSentenceBegin:
			// 句子开始，不需要处理

		case eventResultChanged:
			// 中间结果
			s.resultChan <- asr.Result{
				Text:       resp.Payload.Result,
				Confidence: resp.Payload.Confidence,
				IsFinal:    false,
			}

		case eventSentenceEnd:
			// 句子结束（最终结果）
			s.resultChan <- asr.Result{
				Text:       resp.Payload.Result,
				Confidence: resp.Payload.Confidence,
				IsFinal:    true,
			}

		case eventCompleted:
			// 识别完成
			return

		case eventFailed:
			// 识别失败
			return
		}
	}
}

// response 响应结构
type response struct {
	Header struct {
		Namespace  string `json:"namespace"`
		Name       string `json:"name"`
		Status     int    `json:"status"`
		StatusText string `json:"status_text"`
		TaskID     string `json:"task_id"`
	} `json:"header"`
	Payload struct {
		Index      int     `json:"index"`
		Time       int     `json:"time"`
		BeginTime  int     `json:"begin_time"`
		Result     string  `json:"result"`
		Confidence float64 `json:"confidence"`
	} `json:"payload"`
}
