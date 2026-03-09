package llm

import "context"

// Message 对话消息
type Message struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"`
}

// Tool 工具定义
type Tool struct {
	Type     string       `json:"type"` // function
	Function ToolFunction `json:"function"`
}

// ToolFunction 工具函数定义
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // function
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Response LLM 响应
type Response struct {
	Content      string     `json:"content"`
	FinishReason string     `json:"finish_reason"`
	TokensUsed   int        `json:"tokens_used"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
}

// Chunk 流式响应片段
type Chunk struct {
	Content string
	Done    bool
	Error   error
}

// Provider LLM 服务提供者接口
type Provider interface {
	// Name 返回提供者名称
	Name() string

	// Chat 同步对话
	Chat(ctx context.Context, messages []Message) (*Response, error)

	// ChatWithTools 带工具的同步对话
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*Response, error)

	// ChatStream 流式对话
	ChatStream(ctx context.Context, messages []Message) (<-chan Chunk, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// ProviderConfig 提供者配置
type ProviderConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	ModelFast string
	Proxy     string
	Timeout   int // seconds
}
