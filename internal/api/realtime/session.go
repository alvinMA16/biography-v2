package realtime

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// Session 实时对话会话
type Session struct {
	conn           *websocket.Conn
	conversationID string
	topic          string

	// Providers (通过依赖注入)
	// asr asr.Provider
	// llm llm.Provider
	// tts tts.Provider
}

// NewSession 创建新会话
func NewSession(conn *websocket.Conn, conversationID, topic string) *Session {
	return &Session{
		conn:           conn,
		conversationID: conversationID,
		topic:          topic,
	}
}

// Message WebSocket 消息
type Message struct {
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data,omitempty"`
	Content string          `json:"content,omitempty"`
}

// Run 运行会话主循环
func (s *Session) Run() error {
	// 发送连接成功状态
	s.sendStatus("connected", "已连接")

	// TODO: 发送开场白
	// s.sendGreeting()

	// 主循环：接收并处理消息
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return nil
			}
			return err
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[Session] 无效消息: %v", err)
			continue
		}

		switch msg.Type {
		case "audio":
			// 处理音频数据
			if err := s.handleAudio(msg.Data); err != nil {
				log.Printf("[Session] 处理音频错误: %v", err)
			}

		case "stop":
			// 用户请求停止
			return nil

		default:
			log.Printf("[Session] 未知消息类型: %s", msg.Type)
		}
	}
}

// handleAudio 处理音频数据
func (s *Session) handleAudio(data json.RawMessage) error {
	// TODO: 实现音频处理流程
	// 1. 解码 base64 音频
	// 2. 发送到 ASR 进行识别
	// 3. 识别结果发送到 LLM 生成回复
	// 4. 回复发送到 TTS 生成语音
	// 5. 语音发送给前端

	return nil
}

// sendStatus 发送状态消息
func (s *Session) sendStatus(status, message string) {
	s.sendJSON(map[string]interface{}{
		"type":    "status",
		"status":  status,
		"message": message,
	})
}

// sendText 发送文本消息
func (s *Session) sendText(textType, content string) {
	s.sendJSON(map[string]interface{}{
		"type":      "text",
		"text_type": textType,
		"content":   content,
	})
}

// sendAudio 发送音频数据
func (s *Session) sendAudio(audioData []byte) {
	// TODO: base64 编码后发送
}

// sendJSON 发送 JSON 消息
func (s *Session) sendJSON(v interface{}) {
	if err := s.conn.WriteJSON(v); err != nil {
		log.Printf("[Session] 发送消息失败: %v", err)
	}
}
