package realtime

// MessageType 消息类型
type MessageType string

const (
	// 客户端 → 服务端
	MsgTypeStart MessageType = "start" // 开始对话
	MsgTypeAudio MessageType = "audio" // 音频数据
	MsgTypeStop  MessageType = "stop"  // 停止对话

	// 服务端 → 客户端
	MsgTypeASR      MessageType = "asr"      // ASR 识别结果
	MsgTypeResponse MessageType = "response" // AI 文字回复
	MsgTypeTTS      MessageType = "tts"      // TTS 音频
	MsgTypeDone     MessageType = "done"     // 对话结束
	MsgTypeError    MessageType = "error"    // 错误
)

// Mode 对话模式
type Mode string

const (
	ModeNormal            Mode = "normal"             // 正常话题对话
	ModeProfileCollection Mode = "profile_collection" // 收集用户信息
)

// ClientMessage 客户端消息
type ClientMessage struct {
	Type    MessageType `json:"type"`
	Mode    Mode        `json:"mode,omitempty"`     // start 时指定
	TopicID string      `json:"topic_id,omitempty"` // normal 模式时指定话题
	Data    string      `json:"data,omitempty"`     // audio 时为 base64 PCM
}

// ServerMessage 服务端消息
type ServerMessage struct {
	Type       MessageType `json:"type"`
	Text       string      `json:"text,omitempty"`        // asr/response 时的文字
	Data       string      `json:"data,omitempty"`        // tts 时的 base64 音频
	SampleRate int         `json:"sample_rate,omitempty"` // tts 采样率
	IsFinal    bool        `json:"is_final,omitempty"`    // asr 是否为最终结果
	Error      string      `json:"error,omitempty"`       // 错误信息
}

// SessionConfig 会话配置
type SessionConfig struct {
	Mode           Mode
	TopicID        string
	ConversationID string
	Speaker        string

	// 用户信息（从数据库加载）
	UserID      string
	UserName    string
	BirthYear   *int
	Hometown    string
	MainCity    string
	EraMemories string

	// 话题信息（normal 模式）
	TopicTitle    string
	TopicGreeting string
	TopicContext  string
}
