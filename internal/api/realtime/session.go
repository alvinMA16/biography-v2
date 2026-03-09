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

	"github.com/gorilla/websocket"
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
	writeMu sync.Mutex

	// 依赖
	asrProvider asr.Provider
	llmManager  *llm.Manager
	ttsProvider tts.Provider

	// 对话历史
	messages              []llm.Message
	firstSessionCompleted bool
	mu                    sync.RWMutex

	// 当前用户输入缓冲（由 ASR 消费协程写入）
	currentUserText strings.Builder
	currentASRText  string
	asrTextMu       sync.Mutex

	// ASR 流（一个会话内复用）
	asrAudioChan    chan []byte
	asrMu           sync.Mutex
	audioChunks     int
	audioBytes      int
	audioDropStreak int

	// 上下文取消
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSession 创建新会话
func NewSession(
	conn *websocket.Conn,
	config *SessionConfig,
	asrProvider asr.Provider,
	llmManager *llm.Manager,
	ttsProvider tts.Provider,
) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	return &Session{
		conn:        conn,
		config:      config,
		state:       StateIdle,
		asrProvider: asrProvider,
		llmManager:  llmManager,
		ttsProvider: ttsProvider,
		messages:    make([]llm.Message, 0),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Run 运行会话主循环
func (s *Session) Run() error {
	defer s.cancel()
	defer s.closeASRStream()
	defer func() {
		log.Printf("[Session] 会话结束: chunks=%d, bytes=%d, state=%d", s.audioChunks, s.audioBytes, s.state)
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
			log.Printf("[Session] 收到 stop: 准备结束用户本轮输入")
			// 用户停止说话，生成回复
			if err := s.finishUserTurn(); err != nil {
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
			// 发送 ASR 结果给前端（中间+最终）
			s.sendASR(result.Text, result.IsFinal)
			if strings.TrimSpace(result.Text) != "" {
				log.Printf("[Session] ASR 结果: final=%v len=%d text=%q", result.IsFinal, len(result.Text), truncateForLog(result.Text, 80))
			}

			s.asrTextMu.Lock()
			if result.IsFinal {
				if result.Text != "" {
					s.currentUserText.WriteString(result.Text)
				}
				s.currentASRText = ""
			} else if result.Text != "" {
				// 保留最新中间结果，stop 触发时可兜底使用
				s.currentASRText = result.Text
			}
			s.asrTextMu.Unlock()
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

	s.mu.Lock()
	s.messages = append(s.messages, llm.Message{
		Role:    "system",
		Content: systemPrompt,
	})
	s.mu.Unlock()

	// 发送开场白
	greeting := s.getGreeting()
	if greeting != "" {
		log.Printf("[Session] 发送开场白: len=%d", len(greeting))
		s.mu.Lock()
		s.messages = append(s.messages, llm.Message{
			Role:    "assistant",
			Content: greeting,
		})
		s.mu.Unlock()

		// 发送文字
		s.sendResponse(greeting)

		// TTS 合成开场白
		if err := s.synthesizeAndSendGreeting(greeting); err != nil {
			log.Printf("[Session] TTS 开场白错误: %v", err)
		}
	}

	s.setState(StateListening, "初始化完成，进入监听")
	// 通知前端当前轮次已结束，可以开始录音。
	s.sendDone()
	return nil
}

// handleAudio 处理音频数据
func (s *Session) handleAudio(audioBase64 string) error {
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

// finishUserTurn 完成用户输入，开始生成回复
func (s *Session) finishUserTurn() error {
	// 每轮 stop 后主动结束当前 ASR 会话，避免上游 idle timeout 影响下一轮。
	s.closeASRStream()
	userText := s.collectUserTextWithGrace()

	if userText == "" {
		log.Printf("[Session] stop 收到但未识别到文本，结束本轮并继续监听")
		s.setState(StateListening, "stop 后无识别文本")
		s.sendDone()
		return nil
	}

	log.Printf("[Session] 用户文本确认: len=%d", len(userText))
	s.setState(StateThinking, "用户文本已确认，准备调用 LLM")

	// 添加用户消息
	s.mu.Lock()
	s.messages = append(s.messages, llm.Message{
		Role:    "user",
		Content: userText,
	})
	s.mu.Unlock()

	// 调用 LLM
	provider, err := s.llmManager.Primary()
	if err != nil {
		s.setState(StateListening, "LLM provider 获取失败")
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

	// 模型通过 JSON 协议返回文本或结束指令
	resp, err := provider.Chat(s.ctx, inferenceMessages)

	if err != nil {
		s.setState(StateListening, "LLM 调用失败")
		return fmt.Errorf("LLM chat: %w", err)
	}

	assistantText, shouldEndConversation := decodeAssistantEnvelope(resp.Content, s.config.Mode == ModeFirstSession)
	if shouldEndConversation {
		s.markFirstSessionCompleted()
		log.Printf("[Session] 模型返回结束指令")
	}
	log.Printf("[Session] LLM 回复完成: len=%d", len(assistantText))

	// 添加助手消息
	s.mu.Lock()
	s.messages = append(s.messages, llm.Message{
		Role:    "assistant",
		Content: assistantText,
	})
	s.mu.Unlock()

	// 发送文字响应
	s.sendResponse(assistantText)

	// TTS 合成
	s.setState(StateSpeaking, "LLM 完成，开始 TTS")
	if _, err := s.synthesizeAndSend(assistantText); err != nil {
		log.Printf("[Session] TTS 错误: %v", err)
	}

	// 发送完成信号
	s.sendDone()

	// 如果模型调用了结束工具，发送结束消息给前端
	if shouldEndConversation {
		log.Printf("[Session] 发送 first_session_complete 给前端")
		s.sendFirstSessionComplete()
	}

	s.setState(StateListening, "本轮完成，继续监听")
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

	audioData, err := s.synthesizeAndSend(text)
	if err != nil {
		return err
	}

	storeGreetingAudioToCache(cacheKey, audioData)
	return nil
}

// synthesizeAndSend TTS 合成并发送
func (s *Session) synthesizeAndSend(text string) ([]byte, error) {
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
	var fullAudio []byte
	chunkCount := 0
	for chunk := range audioChan {
		if chunk.Error != nil {
			return nil, fmt.Errorf("TTS stream chunk: %w", chunk.Error)
		}

		if len(chunk.Data) > 0 {
			receivedAnyAudio = true
			chunkCount++
			fullAudio = append(fullAudio, chunk.Data...)
			s.sendTTS(chunk.Data, realtimeTTSSampleRate)
		}

		if chunk.Done {
			break
		}
	}

	if !receivedAnyAudio {
		return nil, fmt.Errorf("TTS stream: no audio data received")
	}
	log.Printf("[Session] TTS 输出完成: chunks=%d bytes=%d", chunkCount, len(fullAudio))

	return fullAudio, nil
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

func (s *Session) sendError(errMsg string) {
	s.sendJSON(ServerMessage{
		Type:  MsgTypeError,
		Error: errMsg,
	})
}

func (s *Session) setState(next SessionState, reason string) {
	prev := s.state
	s.state = next
	log.Printf("[Session] 状态切换: %s -> %s, reason=%s", sessionStateString(prev), sessionStateString(next), reason)
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
func (s *Session) collectUserTextWithGrace() string {
	readBuffers := func() (string, string) {
		s.asrTextMu.Lock()
		defer s.asrTextMu.Unlock()
		return strings.TrimSpace(s.currentUserText.String()), strings.TrimSpace(s.currentASRText)
	}

	initialFinal, initialInterim := readBuffers()
	waitBudget := 250 * time.Millisecond
	if initialFinal != "" || initialInterim != "" {
		waitBudget = 800 * time.Millisecond
	}

	deadline := time.Now().Add(waitBudget)
	for time.Now().Before(deadline) {
		finalText, interimText := readBuffers()
		if finalText != "" && interimText == "" {
			break
		}
		time.Sleep(80 * time.Millisecond)
	}

	s.asrTextMu.Lock()
	defer s.asrTextMu.Unlock()

	finalText := strings.TrimSpace(s.currentUserText.String())
	interimText := strings.TrimSpace(s.currentASRText)
	s.currentUserText.Reset()
	s.currentASRText = ""

	if finalText != "" {
		return finalText
	}
	return interimText
}
