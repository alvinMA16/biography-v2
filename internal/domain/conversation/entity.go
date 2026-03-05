package conversation

import (
	"time"

	"github.com/google/uuid"
)

// Status 对话状态
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusAbandoned Status = "abandoned"
)

// Conversation 对话实体
type Conversation struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"user_id" db:"user_id"`

	// 对话信息
	Title    *string  `json:"title" db:"title"`
	Topic    *string  `json:"topic" db:"topic"`
	Topics   []string `json:"topics" db:"topics"` // 话题标签数组 (JSON)
	Greeting *string  `json:"greeting" db:"greeting"`
	Context  *string  `json:"context" db:"context"`
	Summary  *string  `json:"summary" db:"summary"`

	// 状态
	Status Status `json:"status" db:"status"`

	// 时间戳
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"-" db:"deleted_at"`

	// 关联数据（非数据库字段）
	Messages     []Message `json:"messages,omitempty" db:"-"`
	MessageCount int       `json:"message_count,omitempty" db:"-"`
}

// Message 消息实体
type Message struct {
	ID             uuid.UUID `json:"id" db:"id"`
	ConversationID uuid.UUID `json:"conversation_id" db:"conversation_id"`

	// 消息内容
	Role    string `json:"role" db:"role"`    // user, assistant
	Content string `json:"content" db:"content"`

	// 时间戳
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// CreateConversationInput 创建对话输入
type CreateConversationInput struct {
	Topic    string `json:"topic"`
	Greeting string `json:"greeting"`
	Context  string `json:"context"`
}

// ListConversationsFilter 对话列表过滤条件
type ListConversationsFilter struct {
	UserID uuid.UUID
	Status *Status
	Limit  int
	Offset int
}
