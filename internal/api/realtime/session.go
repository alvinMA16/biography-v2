package realtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"github.com/peizhengma/biography-v2/internal/domain/turntrace"
	"github.com/peizhengma/biography-v2/internal/prompt"
	"github.com/peizhengma/biography-v2/internal/provider/asr"
	"github.com/peizhengma/biography-v2/internal/provider/llm"
	"github.com/peizhengma/biography-v2/internal/provider/tts"
)

// SessionState 会话状态
type SessionState int

const (
	StateIdle      SessionState = iota // 空闲
	StateListening                     // 正在监听用户语音
	StateThinking                      // LLM 生成中
	StateSpeaking                      // TTS 播放中
)

const greetingAudioCacheMaxEntries = 128
const realtimeTTSSampleRate = 16000
const realtimeLLMHedgeAfter = 9 * time.Second
const narrationPersistRuneThreshold = 100
const narrationPersistMaxDelay = 12 * time.Second

var (
	greetingAudioCacheMu sync.RWMutex
	greetingAudioCache   = make(map[string][]byte)
	greetingAudioOrder   []string
)

// Session 实时对话会话
type Session struct {
	conn    *websocket.Conn
	config  *SessionConfig
	state   SessionState
	stateMu sync.RWMutex
	writeMu sync.Mutex

	// 依赖
	asrProvider asr.Provider
	llmManager  *llm.Manager
	ttsProvider tts.Provider
	persistFunc func(role, content string) error

	// 对话历史
	messages              []llm.Message
	firstSessionCompleted bool
	livePersistEnabled    bool
	mu                    sync.RWMutex
	narrationMu           sync.Mutex
	narrationPersistText  string
	narrationBufferSince  *time.Time

	// 当前轮次的 ASR 文本缓冲，按 turn 隔离，避免旧流结果串入新轮。
	currentTurnASRBuffer    *asrTurnBuffer
	finalizingTurnASRBuffer *asrTurnBuffer
	asrTextMu               sync.RWMutex

	// ASR 流（一个会话内复用）
	asrAudioChan    chan []byte
	asrMu           sync.Mutex
	audioChunks     int
	audioBytes      int
	audioDropStreak int

	// 上下文取消
	ctx    context.Context
	cancel context.CancelFunc

	turnIndex             int
	turnDiagnosticPersist func(input *turntrace.CreateInput) error
	audioReceivingSent    bool
}

type asrTurnBuffer struct {
	mu          sync.Mutex
	finalText   strings.Builder
	interimText string
	lastFinalAt *time.Time
}

func (b *asrTurnBuffer) appendResult(text string, isFinal bool) {
	if b == nil || strings.TrimSpace(text) == "" {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if isFinal {
		b.finalText.WriteString(text)
		now := time.Now()
		b.lastFinalAt = &now
		b.interimText = ""
		return
	}

	b.interimText = text
}

func (b *asrTurnBuffer) snapshot() (string, string, *time.Time) {
	if b == nil {
		return "", "", nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	finalText := strings.TrimSpace(b.finalText.String())
	interimText := strings.TrimSpace(b.interimText)
	var lastFinalAt *time.Time
	if b.lastFinalAt != nil {
		t := *b.lastFinalAt
		lastFinalAt = &t
	}
	return finalText, interimText, lastFinalAt
}

func (b *asrTurnBuffer) consume() (string, string, *time.Time) {
	if b == nil {
		return "", "", nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	finalText := strings.TrimSpace(b.finalText.String())
	interimText := strings.TrimSpace(b.interimText)
	var lastFinalAt *time.Time
	if b.lastFinalAt != nil {
		t := *b.lastFinalAt
		lastFinalAt = &t
	}

	b.finalText.Reset()
	b.interimText = ""
	b.lastFinalAt = nil

	return finalText, interimText, lastFinalAt
}

// NewSession 创建新会话
func NewSession(
	conn *websocket.Conn,
	config *SessionConfig,
	asrProvider asr.Provider,
	llmManager *llm.Manager,
	ttsProvider tts.Provider,
	persistFunc func(role, content string) error,
	turnDiagnosticPersist func(input *turntrace.CreateInput) error,
) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		conn:                  conn,
		config:                config,
		state:                 StateIdle,
		asrProvider:           asrProvider,
		llmManager:            llmManager,
		ttsProvider:           ttsProvider,
		persistFunc:           persistFunc,
		messages:              make([]llm.Message, 0),
		livePersistEnabled:    persistFunc != nil,
		ctx:                   ctx,
		cancel:                cancel,
		turnDiagnosticPersist: turnDiagnosticPersist,
	}
}

// Run 运行会话主循环
func (s *Session) Run() error {
	defer s.cancel()
	defer s.closeASRStream()
	defer s.sendTurnStatus(0, TurnStateSessionEnded, "", "")
	defer func() {
		log.Printf("[Session] 会话结束: chunks=%d, bytes=%d, state=%d", s.audioChunks, s.audioBytes, s.getState())
	}()
	log.Printf("[Session] 会话启动: conversation_id=%s mode=%s speaker=%s", s.config.ConversationID, s.config.Mode, s.config.Speaker)

	// 初始化会话
	if err := s.init(); err != nil {
		s.sendError(fmt.Sprintf("初始化失败: %v", err))
		return err
	}

	// 主循环：接收并处理消息
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}
		s.conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[Session] 无效消息: %v", err)
			continue
		}

		switch msg.Type {
		case MsgTypeAudio:
			if err := s.handleAudio(msg.Data); err != nil {
				log.Printf("[Session] 处理音频错误: %v", err)
				s.sendError(err.Error())
			}

		case MsgTypeStop:
			if s.getState() != StateListening {
				log.Printf("[Session] 忽略 stop: current_state=%s", sessionStateString(s.getState()))
				continue
			}
			log.Printf("[Session] 收到 stop: 准备结束用户本轮输入")
			var err error
			if s.isNarrationMode() {
				err = s.finishNarrationTurn()
			} else {
				err = s.finishUserTurn()
			}
			if err != nil {
				log.Printf("[Session] 生成回复错误: %v", err)
				s.sendError(err.Error())
			}

		default:
			log.Printf("[Session] 未知消息类型: %s", msg.Type)
		}
	}
}

func (s *Session) ensureASRStream() error {
	if s.asrProvider == nil {
		return fmt.Errorf("ASR provider not available: set ALIYUN_ACCESS_KEY_ID / ALIYUN_ACCESS_KEY_SECRET / ALIYUN_ASR_APP_KEY")
	}

	s.asrMu.Lock()
	defer s.asrMu.Unlock()

	if s.asrAudioChan != nil {
		return nil
	}

	audioChan := make(chan []byte, 64)
	resultChan, err := s.asrProvider.TranscribeStream(s.ctx, audioChan, "pcm", 16000)
	if err != nil {
		return fmt.Errorf("ASR recognize stream: %w", err)
	}
	log.Printf("[Session] ASR 流已建立: format=pcm sample_rate=16000")

	s.asrAudioChan = audioChan
	turnBuffer := s.ensureCurrentTurnASRBuffer()

	go func() {
		log.Printf("[Session] ASR 结果协程启动")
		defer func(ch chan []byte) {
			// ASR 结果流结束后，回收当前音频流句柄，允许后续自动重建。
			s.asrMu.Lock()
			if s.asrAudioChan == ch {
				log.Printf("[Session] ASR 结果流已结束，回收 ASR 音频流")
				close(s.asrAudioChan)
				s.asrAudioChan = nil
			}
			s.asrMu.Unlock()
		}(audioChan)

		for result := range resultChan {
			if strings.TrimSpace(result.Text) != "" {
				log.Printf("[Session] ASR 结果: final=%v len=%d text=%q", result.IsFinal, len(result.Text), truncateForLog(result.Text, 80))
			}

			turnBuffer.appendResult(result.Text, result.IsFinal)
			if s.isNarrationMode() && result.IsFinal {
				if err := s.queueNarrationText(result.Text); err != nil {
					log.Printf("[Session] 自述内容暂存失败: %v", err)
				}
			}

			// 只把当前 listening turn 的识别结果回显给前端，避免旧流结果污染新轮展示。
			if s.shouldForwardASRResult(turnBuffer) {
				s.sendASR(result.Text, result.IsFinal)
			}
		}
		log.Printf("[Session] ASR 结果协程结束")
	}()

	return nil
}

func (s *Session) closeASRStream() {
	s.asrMu.Lock()
	defer s.asrMu.Unlock()

	if s.asrAudioChan != nil {
		log.Printf("[Session] 关闭 ASR 音频流")
		close(s.asrAudioChan)
		s.asrAudioChan = nil
	}
}

// init 初始化会话
func (s *Session) init() error {
	s.sendTurnStatus(0, TurnStateSessionInitializing, "", "")
	if s.asrProvider != nil {
		if err := s.ensureASRStream(); err != nil {
			return fmt.Errorf("init ASR stream: %w", err)
		}
	}

	// 构建系统 prompt
	systemPrompt, err := s.buildSystemPrompt()
	if err != nil {
		return fmt.Errorf("build system prompt: %w", err)
	}

	if strings.TrimSpace(systemPrompt) != "" {
		s.appendConversationMessage("system", systemPrompt)
	}

	// 发送开场白
	greeting := s.getGreeting()
	if greeting != "" {
		s.sendTurnStatus(0, TurnStateGreetingPreparing, "", "")
		log.Printf("[Session] 发送开场白: len=%d", len(greeting))
		if !s.isNarrationMode() {
			s.appendConversationMessage("assistant", greeting)
			s.tryPersistMessage("assistant", greeting)
		}

		// 发送文字
		s.sendResponse(greeting)

		// TTS 合成开场白
		s.sendTurnStatus(0, TurnStateGreetingSpeaking, "tts", "")
		if err := s.synthesizeAndSendGreeting(greeting); err != nil {
			log.Printf("[Session] TTS 开场白错误: %v", err)
		}
	}

	s.setState(StateListening, "初始化完成，进入监听")
	// 通知前端当前轮次已结束，可以开始录音。
	s.sendDone()
	s.sendTurnStatus(0, TurnStateReadyForUser, "", "")
	return nil
}

func (s *Session) finishNarrationTurn() error {
	turnState := s.newTurnDiagnostic(time.Now())
	s.audioReceivingSent = false
	s.sendTurnStatus(turnState.turnIndex, TurnStateUserStopReceived, "", "")

	turnBuffer := s.beginTurnASRFinalization()
	s.closeASRStream()
	s.sendTurnStatus(turnState.turnIndex, TurnStateASRFinalizing, "asr", "")

	userText, userTextSource, asrFinalAt := s.collectUserTextWithGrace(turnBuffer)
	turnState.userText = userText
	turnState.userTextSource = userTextSource
	turnState.asrFinalAt = asrFinalAt

	switch userTextSource {
	case turntrace.UserTextSourceFinal:
		s.sendTurnStatus(turnState.turnIndex, TurnStateASRFinalReceived, "asr", "")
	case turntrace.UserTextSourceInterim:
		s.sendTurnStatus(turnState.turnIndex, TurnStateASRInterimFallback, "asr", "")
		if err := s.queueNarrationText(userText); err != nil {
			log.Printf("[Session] 追加自述 fallback 文本失败: %v", err)
		}
	default:
		s.sendTurnStatus(turnState.turnIndex, TurnStateASREmpty, "asr", "")
	}

	if _, err := s.flushNarrationBuffer(true, false); err != nil {
		log.Printf("[Session] 自述分批落库失败，将在后续继续重试: %v", err)
	}

	doneAt := time.Now()
	s.sendDone()
	turnState.doneSentAt = &doneAt
	s.sendTurnStatus(turnState.turnIndex, TurnStateTurnDoneSent, "", "")
	s.persistTurnDiagnostic(turnState)

	s.setState(StateListening, "自述分段完成，继续监听")
	s.sendTurnStatus(turnState.turnIndex, TurnStateReadyForUser, "", "")
	return nil
}

// handleAudio 处理音频数据
func (s *Session) handleAudio(audioBase64 string) error {
	if s.getState() != StateListening {
		return nil
	}

	if err := s.ensureASRStream(); err != nil {
		return err
	}

	// 解码 base64 音频
	audioData, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		return fmt.Errorf("decode audio: %w", err)
	}

	s.asrMu.Lock()
	audioChan := s.asrAudioChan
	s.asrMu.Unlock()
	if audioChan == nil {
		return fmt.Errorf("ASR stream not ready")
	}

	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case audioChan <- audioData:
		s.audioDropStreak = 0
		if !s.audioReceivingSent {
			s.audioReceivingSent = true
			s.sendTurnStatus(s.turnIndex+1, TurnStateUserAudioReceiving, "", "")
		}
	case <-time.After(80 * time.Millisecond):
		// 不阻塞主循环，音频拥塞时优先丢包并继续会话。
		s.audioDropStreak++
		if s.audioDropStreak == 1 || s.audioDropStreak%20 == 0 {
			log.Printf("[Session] ASR 音频队列拥塞，丢弃音频块: streak=%d", s.audioDropStreak)
		}
		// 连续拥塞说明下游可能失活，主动重建 ASR 流。
		if s.audioDropStreak >= 80 {
			log.Printf("[Session] ASR 音频连续拥塞，重置 ASR 流")
			s.audioDropStreak = 0
			s.closeASRStream()
		}
		return nil
	}

	s.audioChunks++
	s.audioBytes += len(audioData)
	if s.audioChunks%50 == 0 {
		avg := 0
		if s.audioChunks > 0 {
			avg = s.audioBytes / s.audioChunks
		}
		log.Printf("[Session] 音频接收中: chunks=%d, bytes=%d, avg_chunk_bytes=%d", s.audioChunks, s.audioBytes, avg)
	}

	return nil
}

type ttsSynthesisResult struct {
	audio        []byte
	firstChunkAt *time.Time
	completedAt  *time.Time
}

type turnDiagnosticState struct {
	turnIndex       int
	outcome         turntrace.Outcome
	userTextSource  turntrace.UserTextSource
	userText        string
	assistantText   string
	userStopAt      time.Time
	asrFinalAt      *time.Time
	llmStartedAt    *time.Time
	llmCompletedAt  *time.Time
	ttsStartedAt    *time.Time
	ttsFirstChunkAt *time.Time
	ttsCompletedAt  *time.Time
	doneSentAt      *time.Time
	errorStage      *string
	errorMessage    *string
}

func (s *Session) newTurnDiagnostic(stopAt time.Time) *turnDiagnosticState {
	s.turnIndex++
	return &turnDiagnosticState{
		turnIndex:      s.turnIndex,
		outcome:        turntrace.OutcomeCompleted,
		userTextSource: turntrace.UserTextSourceEmpty,
		userStopAt:     stopAt,
	}
}

func (s *Session) persistTurnDiagnostic(state *turnDiagnosticState) {
	if state == nil || s.turnDiagnosticPersist == nil {
		return
	}

	input := &turntrace.CreateInput{
		TurnIndex:        state.turnIndex,
		Mode:             string(s.config.Mode),
		Outcome:          state.outcome,
		UserTextSource:   state.userTextSource,
		UserTextPreview:  strings.TrimSpace(state.userText),
		UserTextLength:   len([]rune(strings.TrimSpace(state.userText))),
		AssistantPreview: strings.TrimSpace(state.assistantText),
		AssistantLength:  len([]rune(strings.TrimSpace(state.assistantText))),
		UserStopAt:       state.userStopAt,
		ASRFinalAt:       state.asrFinalAt,
		LLMStartedAt:     state.llmStartedAt,
		LLMCompletedAt:   state.llmCompletedAt,
		TTSStartedAt:     state.ttsStartedAt,
		TTSFirstChunkAt:  state.ttsFirstChunkAt,
		TTSCompletedAt:   state.ttsCompletedAt,
		DoneSentAt:       state.doneSentAt,
		ErrorStage:       state.errorStage,
		ErrorMessage:     state.errorMessage,
	}
	if err := s.turnDiagnosticPersist(input); err != nil {
		log.Printf("[Session] 保存轮次诊断失败: turn=%d err=%v", state.turnIndex, err)
	}
}

// finishUserTurn 完成用户输入，开始生成回复
func (s *Session) finishUserTurn() error {
	turnState := s.newTurnDiagnostic(time.Now())
	s.audioReceivingSent = false
	s.sendTurnStatus(turnState.turnIndex, TurnStateUserStopReceived, "", "")

	turnBuffer := s.beginTurnASRFinalization()

	// 每轮 stop 后主动结束当前 ASR 会话，避免上游 idle timeout 影响下一轮。
	s.closeASRStream()
	s.sendTurnStatus(turnState.turnIndex, TurnStateASRFinalizing, "asr", "")
	userText, userTextSource, asrFinalAt := s.collectUserTextWithGrace(turnBuffer)
	turnState.userText = userText
	turnState.userTextSource = userTextSource
	turnState.asrFinalAt = asrFinalAt

	if userText == "" {
		log.Printf("[Session] stop 收到但未识别到文本，结束本轮并继续监听")
		s.setState(StateListening, "stop 后无识别文本")
		s.sendTurnStatus(turnState.turnIndex, TurnStateASREmpty, "asr", "")
		doneAt := time.Now()
		s.sendDone()
		turnState.outcome = turntrace.OutcomeEmptyInput
		turnState.doneSentAt = &doneAt
		s.sendTurnStatus(turnState.turnIndex, TurnStateTurnDoneSent, "", "")
		s.sendTurnStatus(turnState.turnIndex, TurnStateReadyForUser, "", "")
		s.persistTurnDiagnostic(turnState)
		return nil
	}

	switch userTextSource {
	case turntrace.UserTextSourceFinal:
		s.sendTurnStatus(turnState.turnIndex, TurnStateASRFinalReceived, "asr", "")
	case turntrace.UserTextSourceInterim:
		s.sendTurnStatus(turnState.turnIndex, TurnStateASRInterimFallback, "asr", "")
	}

	log.Printf("[Session] 用户文本确认: len=%d", len(userText))
	s.setState(StateThinking, "用户文本已确认，准备调用 LLM")
	llmStartedAt := time.Now()
	turnState.llmStartedAt = &llmStartedAt
	s.sendTurnStatus(turnState.turnIndex, TurnStateLLMRequestStarted, "llm", "")

	// 添加用户消息
	s.mu.Lock()
	s.messages = append(s.messages, llm.Message{
		Role:    "user",
		Content: userText,
	})
	s.mu.Unlock()
	s.tryPersistMessage("user", userText)

	// 调用 LLM
	provider, err := s.llmManager.Primary()
	if err != nil {
		s.setState(StateListening, "LLM provider 获取失败")
		turnState.outcome = turntrace.OutcomeLLMError
		turnState.errorStage = stringPtr("llm")
		turnState.errorMessage = stringPtr(err.Error())
		s.sendTurnStatus(turnState.turnIndex, TurnStateTurnFailed, "llm", err.Error())
		s.persistTurnDiagnostic(turnState)
		return fmt.Errorf("LLM not available: %w", err)
	}

	s.mu.RLock()
	allMessages := make([]llm.Message, len(s.messages))
	copy(allMessages, s.messages)
	s.mu.RUnlock()
	packet := buildChatContextPacket(s.config, allMessages)
	systemPrompt := ""
	if len(allMessages) > 0 && allMessages[0].Role == "system" {
		systemPrompt = allMessages[0].Content
	}
	inferenceMessages := buildInferenceMessages(packet, systemPrompt)
	log.Printf(
		"[Session] 调用 LLM: provider=%s raw_messages=%d inference_messages=%d current_turn=%s",
		provider.Name(),
		len(allMessages),
		len(inferenceMessages),
		formatTurnContextForLog(packet.CurrentUserTurn),
	)

	// 实时链路优先压缩长尾；Gemini 主模型超过 9 秒时启动对冲请求。
	var (
		resp         *llm.Response
		usedProvider string
	)
	if provider.Name() == "gemini" {
		resp, usedProvider, err = s.llmManager.HedgedChat(s.ctx, inferenceMessages, llm.HedgedChatConfig{
			HedgeAfter: realtimeLLMHedgeAfter,
			HedgeProviders: []string{
				llm.ProviderGeminiRealtimePreview,
				llm.ProviderGeminiRealtimeFast,
			},
		})
	} else {
		resp, usedProvider, err = s.llmManager.ChatWithRetry(s.ctx, inferenceMessages, 3)
	}
	if err != nil {
		s.setState(StateListening, "LLM 调用失败")
		turnState.outcome = turntrace.OutcomeLLMError
		turnState.errorStage = stringPtr("llm")
		turnState.errorMessage = stringPtr(err.Error())
		s.sendTurnStatus(turnState.turnIndex, TurnStateTurnFailed, "llm", err.Error())
		s.persistTurnDiagnostic(turnState)
		return fmt.Errorf("LLM chat: %w", err)
	}
	llmCompletedAt := time.Now()
	turnState.llmCompletedAt = &llmCompletedAt
	s.sendTurnStatus(turnState.turnIndex, TurnStateLLMResponseReceived, "llm", "")
	if usedProvider != provider.Name() {
		log.Printf("[Session] LLM 实际使用 provider=%s", usedProvider)
	}

	assistantText := strings.TrimSpace(resp.Content)
	shouldEndConversation := false
	if s.config.Mode == ModeFirstSession {
		assistantText, shouldEndConversation = decodeAssistantEnvelope(resp.Content, true)
		if shouldEndConversation {
			s.markFirstSessionCompleted()
			log.Printf("[Session] 模型返回结束指令")
		}
	}
	log.Printf("[Session] LLM 回复完成: len=%d", len(assistantText))
	turnState.assistantText = assistantText

	// 添加助手消息
	s.mu.Lock()
	s.messages = append(s.messages, llm.Message{
		Role:    "assistant",
		Content: assistantText,
	})
	s.mu.Unlock()
	s.tryPersistMessage("assistant", assistantText)

	// 发送文字响应
	s.sendResponse(assistantText)
	s.sendTurnStatus(turnState.turnIndex, TurnStateAssistantSent, "", "")

	// TTS 合成
	s.setState(StateSpeaking, "LLM 完成，开始 TTS")
	if s.ttsProvider != nil {
		ttsStartedAt := time.Now()
		turnState.ttsStartedAt = &ttsStartedAt
		s.sendTurnStatus(turnState.turnIndex, TurnStateTTSRequestStarted, "tts", "")
	}
	ttsResult, err := s.synthesizeAndSend(assistantText)
	if err != nil {
		log.Printf("[Session] TTS 错误: %v", err)
		turnState.outcome = turntrace.OutcomeTTSError
		turnState.errorStage = stringPtr("tts")
		turnState.errorMessage = stringPtr(err.Error())
		s.sendTurnStatus(turnState.turnIndex, TurnStateTurnFailed, "tts", err.Error())
	} else if ttsResult != nil {
		turnState.ttsFirstChunkAt = ttsResult.firstChunkAt
		turnState.ttsCompletedAt = ttsResult.completedAt
		s.sendTurnStatus(turnState.turnIndex, TurnStateTTSCompleted, "tts", "")
	}

	// 发送完成信号
	doneAt := time.Now()
	s.sendDone()
	turnState.doneSentAt = &doneAt
	s.sendTurnStatus(turnState.turnIndex, TurnStateTurnDoneSent, "", "")
	s.persistTurnDiagnostic(turnState)

	// 如果模型调用了结束工具，发送结束消息给前端
	if shouldEndConversation {
		log.Printf("[Session] 发送 first_session_complete 给前端")
		s.sendFirstSessionComplete()
	}

	s.setState(StateListening, "本轮完成，继续监听")
	s.sendTurnStatus(turnState.turnIndex, TurnStateReadyForUser, "", "")
	return nil
}

func ensureFirstSessionClosingText(content string, shouldEnd bool) string {
	if !shouldEnd || strings.TrimSpace(content) != "" {
		return strings.TrimSpace(content)
	}
	return "好的，那我们先聊到这儿，以后咱们再慢慢聊您的人生故事。"
}

type assistantEnvelope struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Tool    string `json:"tool,omitempty"`
}

func decodeAssistantEnvelope(raw string, allowTool bool) (string, bool) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", false
	}

	jsonText := unwrapJSONCodeFence(cleaned)
	var envelope assistantEnvelope
	if err := json.Unmarshal([]byte(jsonText), &envelope); err != nil {
		log.Printf("[Session] 模型未返回有效 JSON，按普通文本兜底: err=%v", err)
		return fallbackAssistantText(cleaned), false
	}

	content := strings.TrimSpace(envelope.Content)
	msgType := strings.ToLower(strings.TrimSpace(envelope.Type))
	toolName := strings.ToLower(strings.TrimSpace(envelope.Tool))

	switch msgType {
	case "text":
		if content != "" {
			return content, false
		}
	case "tool":
		if allowTool && toolName == "end_conversation" {
			return ensureFirstSessionClosingText(content, true), true
		}
		if content != "" {
			log.Printf("[Session] 忽略未授权工具指令: type=%s tool=%s", msgType, toolName)
			return content, false
		}
	}

	log.Printf("[Session] 模型 JSON 缺少有效内容，按兜底文案处理: type=%s tool=%s", msgType, toolName)
	return fallbackAssistantText(cleaned), false
}

func unwrapJSONCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}

	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return trimmed
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func fallbackAssistantText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "您接着说，我在听。"
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "您接着说，我在听。"
	}
	return trimmed
}

// buildSystemPrompt 构建系统 prompt
func (s *Session) buildSystemPrompt() (string, error) {
	if s.isNarrationMode() {
		return "", nil
	}

	var tmplStr string
	if s.config.Mode == ModeFirstSession {
		tmplStr = prompt.FirstSessionSystemPrompt
	} else {
		tmplStr = prompt.RealtimeChatSystemPrompt
	}

	tmpl, err := template.New("system").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	birthYear := 0
	if s.config.BirthYear != nil {
		birthYear = *s.config.BirthYear
	}

	data := map[string]interface{}{
		"UserName":       s.config.UserName,
		"BirthYear":      birthYear,
		"Hometown":       s.config.Hometown,
		"MainCity":       s.config.MainCity,
		"EraMemories":    s.config.EraMemories,
		"TopicTitle":     s.config.TopicTitle,
		"TopicContext":   s.config.TopicContext,
		"RecorderName":   s.config.RecorderName,
		"RecorderGender": s.config.RecorderGender,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getGreeting 获取开场白
func (s *Session) getGreeting() string {
	if s.isNarrationMode() {
		return prompt.NarrationGreeting
	}
	if s.config.Mode == ModeFirstSession {
		// 根据记录师性别选择对应的开场白
		if s.config.RecorderGender == "male" {
			return prompt.FirstSessionGreetingMale
		}
		return prompt.FirstSessionGreetingFemale
	}
	if s.config.TopicGreeting != "" {
		return s.config.TopicGreeting
	}
	return ""
}

func (s *Session) isNarrationMode() bool {
	return s != nil && s.config != nil && s.config.Mode == ModeNarration
}

func (s *Session) appendConversationMessage(role, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, llm.Message{
		Role:    role,
		Content: content,
	})
}

func shouldFlushNarrationBatch(text string, startedAt *time.Time, now time.Time, force bool) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if force {
		return true
	}
	if utf8.RuneCountInString(text) >= narrationPersistRuneThreshold {
		return true
	}
	if startedAt != nil && now.Sub(*startedAt) >= narrationPersistMaxDelay {
		return true
	}
	return false
}

func (s *Session) queueNarrationText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	now := time.Now()

	s.narrationMu.Lock()
	if s.narrationPersistText == "" {
		s.narrationPersistText = text
		s.narrationBufferSince = &now
	} else {
		s.narrationPersistText += "\n" + text
	}
	persistText := s.narrationPersistText
	startedAt := s.narrationBufferSince
	s.narrationMu.Unlock()

	if shouldFlushNarrationBatch(persistText, startedAt, now, false) {
		_, err := s.flushNarrationBuffer(false, false)
		return err
	}
	return nil
}

func (s *Session) flushNarrationBuffer(force bool, fallbackToMemory bool) (bool, error) {
	if !s.isNarrationMode() {
		return false, nil
	}

	now := time.Now()

	s.narrationMu.Lock()
	text := strings.TrimSpace(s.narrationPersistText)
	startedAt := s.narrationBufferSince
	if !shouldFlushNarrationBatch(text, startedAt, now, force) {
		s.narrationMu.Unlock()
		return false, nil
	}
	s.narrationMu.Unlock()

	persist := s.persistFunc
	if persist != nil {
		if err := persist("user", text); err != nil {
			if fallbackToMemory {
				s.appendConversationMessage("user", text)
				s.clearNarrationPersistedText(text)
				return true, nil
			}
			return false, err
		}
	}

	s.appendConversationMessage("user", text)
	s.clearNarrationPersistedText(text)
	return true, nil
}

func (s *Session) clearNarrationPersistedText(batch string) {
	s.narrationMu.Lock()
	defer s.narrationMu.Unlock()

	current := strings.TrimSpace(s.narrationPersistText)
	if current == "" {
		s.narrationPersistText = ""
		s.narrationBufferSince = nil
		return
	}

	switch {
	case current == batch:
		s.narrationPersistText = ""
	case strings.HasPrefix(current, batch+"\n"):
		s.narrationPersistText = strings.TrimPrefix(current, batch+"\n")
	case strings.HasPrefix(current, batch):
		s.narrationPersistText = strings.TrimSpace(strings.TrimPrefix(current, batch))
	}

	if strings.TrimSpace(s.narrationPersistText) == "" {
		s.narrationPersistText = ""
		s.narrationBufferSince = nil
		return
	}

	now := time.Now()
	s.narrationBufferSince = &now
}

func (s *Session) PrepareNarrationMessagesForSave() {
	if !s.isNarrationMode() {
		return
	}
	if _, err := s.flushNarrationBuffer(true, true); err != nil {
		log.Printf("[Session] 自述内容收尾落库失败，回退内存补漏: %v", err)
	}
}

func (s *Session) greetingCacheKey(text string) string {
	return fmt.Sprintf("%s|%s|%d|%s|%s", s.ttsProvider.Name(), s.config.Speaker, realtimeTTSSampleRate, "pcm", text)
}

func getGreetingAudioFromCache(key string) ([]byte, bool) {
	greetingAudioCacheMu.RLock()
	defer greetingAudioCacheMu.RUnlock()

	audio, ok := greetingAudioCache[key]
	if !ok {
		return nil, false
	}

	// 返回副本，避免并发场景下外部修改底层切片。
	audioCopy := make([]byte, len(audio))
	copy(audioCopy, audio)
	return audioCopy, true
}

func storeGreetingAudioToCache(key string, audio []byte) {
	if len(audio) == 0 {
		return
	}

	audioCopy := make([]byte, len(audio))
	copy(audioCopy, audio)

	greetingAudioCacheMu.Lock()
	defer greetingAudioCacheMu.Unlock()

	if _, exists := greetingAudioCache[key]; !exists {
		greetingAudioOrder = append(greetingAudioOrder, key)
	}
	greetingAudioCache[key] = audioCopy

	for len(greetingAudioOrder) > greetingAudioCacheMaxEntries {
		evictKey := greetingAudioOrder[0]
		greetingAudioOrder = greetingAudioOrder[1:]
		delete(greetingAudioCache, evictKey)
	}
}

func (s *Session) synthesizeAndSendGreeting(text string) error {
	if s.ttsProvider == nil {
		return nil
	}

	cacheKey := s.greetingCacheKey(text)
	if cachedAudio, ok := getGreetingAudioFromCache(cacheKey); ok {
		s.sendTTS(cachedAudio, realtimeTTSSampleRate)
		return nil
	}

	result, err := s.synthesizeAndSend(text)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}

	storeGreetingAudioToCache(cacheKey, result.audio)
	return nil
}

// synthesizeAndSend TTS 合成并发送
func (s *Session) synthesizeAndSend(text string) (*ttsSynthesisResult, error) {
	if s.ttsProvider == nil {
		return nil, nil // TTS 不可用时静默跳过
	}

	audioChan, err := s.ttsProvider.SynthesizeStream(s.ctx, text, tts.SynthesisConfig{
		Voice:      s.config.Speaker,
		SampleRate: realtimeTTSSampleRate,
		Format:     "pcm",
	})
	if err != nil {
		return nil, fmt.Errorf("TTS synthesize stream: %w", err)
	}

	receivedAnyAudio := false
	result := &ttsSynthesisResult{}
	chunkCount := 0
	for chunk := range audioChan {
		if chunk.Error != nil {
			return nil, fmt.Errorf("TTS stream chunk: %w", chunk.Error)
		}

		if len(chunk.Data) > 0 {
			receivedAnyAudio = true
			chunkCount++
			if result.firstChunkAt == nil {
				firstChunkAt := time.Now()
				result.firstChunkAt = &firstChunkAt
				s.sendTurnStatus(s.turnIndex, TurnStateTTSFirstChunkSent, "tts", "")
			}
			result.audio = append(result.audio, chunk.Data...)
			s.sendTTS(chunk.Data, realtimeTTSSampleRate)
		}

		if chunk.Done {
			completedAt := time.Now()
			result.completedAt = &completedAt
			break
		}
	}

	if !receivedAnyAudio {
		return nil, fmt.Errorf("TTS stream: no audio data received")
	}
	if result.completedAt == nil {
		completedAt := time.Now()
		result.completedAt = &completedAt
	}
	log.Printf("[Session] TTS 输出完成: chunks=%d bytes=%d", chunkCount, len(result.audio))

	return result, nil
}

// GetMessages 获取对话历史
func (s *Session) GetMessages() []llm.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := make([]llm.Message, len(s.messages))
	copy(messages, s.messages)
	return messages
}

// GetConversationText 获取对话文本（不含系统 prompt）
func (s *Session) GetConversationText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	for _, msg := range s.messages {
		if msg.Role == "system" {
			continue
		}
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		sb.WriteString(fmt.Sprintf("%s：%s\n\n", role, msg.Content))
	}
	return sb.String()
}

// === 发送消息方法 ===

func (s *Session) sendJSON(v interface{}) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.WriteJSON(v); err != nil {
		log.Printf("[Session] 发送消息失败: %v", err)
	}
}

func (s *Session) sendASR(text string, isFinal bool) {
	s.sendJSON(ServerMessage{
		Type:    MsgTypeASR,
		Text:    text,
		IsFinal: isFinal,
	})
}

func (s *Session) sendResponse(text string) {
	s.sendJSON(ServerMessage{
		Type: MsgTypeResponse,
		Text: text,
	})
}

func (s *Session) sendTTS(audio []byte, sampleRate int) {
	s.sendJSON(ServerMessage{
		Type:       MsgTypeTTS,
		Data:       base64.StdEncoding.EncodeToString(audio),
		SampleRate: sampleRate,
	})
}

func (s *Session) sendDone() {
	log.Printf("[Session] 向前端发送 done")
	s.sendJSON(ServerMessage{
		Type: MsgTypeDone,
	})
}

func (s *Session) sendTurnStatus(turnID int, state TurnState, stage string, message string) {
	s.sendJSON(ServerMessage{
		Type:    MsgTypeTurnStatus,
		TurnID:  turnID,
		State:   state,
		Stage:   stage,
		Message: message,
		At:      time.Now().Format(time.RFC3339Nano),
	})
}

func (s *Session) sendFirstSessionComplete() {
	s.sendJSON(ServerMessage{
		Type: MsgTypeFirstSessionComplete,
	})
}

func (s *Session) markFirstSessionCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firstSessionCompleted = true
}

func (s *Session) FirstSessionCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.firstSessionCompleted
}

func (s *Session) tryPersistMessage(role, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	s.mu.Lock()
	if !s.livePersistEnabled || s.persistFunc == nil {
		s.mu.Unlock()
		return
	}
	persist := s.persistFunc
	s.mu.Unlock()

	if err := persist(role, content); err != nil {
		log.Printf("[Session] 实时保存消息失败，后续改为结束时补漏: role=%s err=%v", role, err)
		s.mu.Lock()
		s.livePersistEnabled = false
		s.mu.Unlock()
	}
}

func (s *Session) sendError(errMsg string) {
	s.sendJSON(ServerMessage{
		Type:  MsgTypeError,
		Error: errMsg,
	})
}

func (s *Session) getState() SessionState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

func (s *Session) setState(next SessionState, reason string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	prev := s.state
	s.state = next
	log.Printf("[Session] 状态切换: %s -> %s, reason=%s", sessionStateString(prev), sessionStateString(next), reason)
}

func (s *Session) ensureCurrentTurnASRBuffer() *asrTurnBuffer {
	s.asrTextMu.Lock()
	defer s.asrTextMu.Unlock()
	if s.currentTurnASRBuffer == nil {
		s.currentTurnASRBuffer = &asrTurnBuffer{}
	}
	return s.currentTurnASRBuffer
}

func (s *Session) beginTurnASRFinalization() *asrTurnBuffer {
	s.asrTextMu.Lock()
	defer s.asrTextMu.Unlock()

	if s.currentTurnASRBuffer == nil {
		s.currentTurnASRBuffer = &asrTurnBuffer{}
	}
	s.finalizingTurnASRBuffer = s.currentTurnASRBuffer
	s.currentTurnASRBuffer = nil
	return s.finalizingTurnASRBuffer
}

func (s *Session) finishTurnASRFinalization(buffer *asrTurnBuffer) {
	if buffer == nil {
		return
	}

	s.asrTextMu.Lock()
	defer s.asrTextMu.Unlock()
	if s.finalizingTurnASRBuffer == buffer {
		s.finalizingTurnASRBuffer = nil
	}
}

func (s *Session) shouldForwardASRResult(buffer *asrTurnBuffer) bool {
	if buffer == nil || s.getState() != StateListening {
		return false
	}

	s.asrTextMu.RLock()
	defer s.asrTextMu.RUnlock()
	return s.currentTurnASRBuffer == buffer
}

func sessionStateString(state SessionState) string {
	switch state {
	case StateIdle:
		return "idle"
	case StateListening:
		return "listening"
	case StateThinking:
		return "thinking"
	case StateSpeaking:
		return "speaking"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// stop 后给 ASR 一小段收尾时间，尽量拿到 SentenceEnd 最终文本，减少尾字丢失。
func (s *Session) collectUserTextWithGrace(buffer *asrTurnBuffer) (string, turntrace.UserTextSource, *time.Time) {
	defer s.finishTurnASRFinalization(buffer)

	readBuffers := func() (string, string, *time.Time) {
		return buffer.snapshot()
	}

	initialFinal, initialInterim, _ := readBuffers()
	waitBudget := 250 * time.Millisecond
	if initialFinal != "" || initialInterim != "" {
		waitBudget = 800 * time.Millisecond
	}

	deadline := time.Now().Add(waitBudget)
	for time.Now().Before(deadline) {
		finalText, interimText, _ := readBuffers()
		if finalText != "" && interimText == "" {
			break
		}
		time.Sleep(80 * time.Millisecond)
	}

	finalText, interimText, asrFinalAt := buffer.consume()

	if finalText != "" {
		return finalText, turntrace.UserTextSourceFinal, asrFinalAt
	}
	if interimText != "" {
		return interimText, turntrace.UserTextSourceInterim, asrFinalAt
	}
	return "", turntrace.UserTextSourceEmpty, asrFinalAt
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := value
	return &v
}
