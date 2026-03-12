package realtime

// MessageType 消息类型
type MessageType string

const (
	// 客户端 → 服务端
	MsgTypeStart MessageType = "start" // 开始对话
	MsgTypeAudio MessageType = "audio" // 音频数据
	MsgTypeStop  MessageType = "stop"  // 停止对话

	// 服务端 → 客户端
	MsgTypeTurnStatus           MessageType = "turn_status"            // 轮次状态事件
	MsgTypeASR                  MessageType = "asr"                    // ASR 识别结果
	MsgTypeResponse             MessageType = "response"               // AI 文字回复
	MsgTypeTTS                  MessageType = "tts"                    // TTS 音频
	MsgTypeDone                 MessageType = "done"                   // 对话结束
	MsgTypeError                MessageType = "error"                  // 错误
	MsgTypeFirstSessionComplete MessageType = "first_session_complete" // 首次对话完成
)

// TurnState 轮次状态
type TurnState string

const (
	TurnStateSessionInitializing TurnState = "session_initializing"
	TurnStateGreetingPreparing   TurnState = "greeting_preparing"
	TurnStateGreetingSpeaking    TurnState = "greeting_speaking"
	TurnStateReadyForUser        TurnState = "ready_for_user"
	TurnStateUserAudioReceiving  TurnState = "user_audio_receiving"
	TurnStateUserStopReceived    TurnState = "user_stop_received"
	TurnStateASRFinalizing       TurnState = "asr_finalizing"
	TurnStateASRFinalReceived    TurnState = "asr_final_received"
	TurnStateASRInterimFallback  TurnState = "asr_interim_fallback"
	TurnStateASREmpty            TurnState = "asr_empty"
	TurnStateLLMRequestStarted   TurnState = "llm_request_started"
	TurnStateLLMResponseReceived TurnState = "llm_response_received"
	TurnStateAssistantSent       TurnState = "assistant_response_sent"
	TurnStateTTSRequestStarted   TurnState = "tts_request_started"
	TurnStateTTSFirstChunkSent   TurnState = "tts_first_chunk_sent"
	TurnStateTTSCompleted        TurnState = "tts_completed"
	TurnStateTurnDoneSent        TurnState = "turn_done_sent"
	TurnStateTurnFailed          TurnState = "turn_failed"
	TurnStateSessionEnded        TurnState = "session_ended"
)

// Mode 对话模式
type Mode string

const (
	ModeNormal       Mode = "normal"        // 正常话题对话
	ModeFirstSession Mode = "first_session" // 首次对话
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
	TurnID     int         `json:"turn_id,omitempty"`     // 当前轮次，0 表示会话级/开场
	State      TurnState   `json:"state,omitempty"`       // turn_status 状态
	Stage      string      `json:"stage,omitempty"`       // 失败阶段或补充阶段
	Message    string      `json:"message,omitempty"`     // turn_status 补充信息
	At         string      `json:"at,omitempty"`          // RFC3339Nano 时间戳
}

// SessionConfig 会话配置
type SessionConfig struct {
	Mode           Mode
	TopicID        string
	ConversationID string
	Speaker        string

	// 记录师信息
	RecorderName   string // 记录师名字（忆安/言川）
	RecorderGender string // 记录师性别（female/male）

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
