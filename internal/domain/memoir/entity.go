package memoir

import (
	"time"

	"github.com/google/uuid"
)

// MemoirStatus 回忆录状态
type MemoirStatus string

const (
	MemoirStatusGenerating MemoirStatus = "generating"
	MemoirStatusCompleted  MemoirStatus = "completed"
)

// Memoir 回忆录实体
type Memoir struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	ConversationID *uuid.UUID `json:"conversation_id" db:"conversation_id"`

	// 内容
	Title   string `json:"title" db:"title"`
	Content string `json:"content" db:"content"`

	// 状态
	Status              MemoirStatus `json:"status" db:"status"`
	SourceConversations []uuid.UUID  `json:"source_conversations" db:"source_conversations"` // 来源对话ID数组 (JSON)

	// 时间线信息
	TimePeriod *string `json:"time_period" db:"time_period"`
	StartYear  *int    `json:"start_year" db:"start_year"`
	EndYear    *int    `json:"end_year" db:"end_year"`
	SortOrder  int     `json:"sort_order" db:"sort_order"`

	// 时间戳
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"-" db:"deleted_at"`
}

// CreateMemoirInput 创建回忆录输入
type CreateMemoirInput struct {
	ConversationID *uuid.UUID `json:"conversation_id"`
	Title          string     `json:"title" binding:"required"`
	Content        string     `json:"content" binding:"required"`
	TimePeriod     string     `json:"time_period"`
	StartYear      *int       `json:"start_year"`
	EndYear        *int       `json:"end_year"`
}

// UpdateMemoirInput 更新回忆录输入
type UpdateMemoirInput struct {
	Title      *string `json:"title"`
	Content    *string `json:"content"`
	TimePeriod *string `json:"time_period"`
	StartYear  *int    `json:"start_year"`
	EndYear    *int    `json:"end_year"`
	SortOrder  *int    `json:"sort_order"`
}

// ListMemoirsFilter 回忆录列表过滤条件
type ListMemoirsFilter struct {
	UserID uuid.UUID
	Limit  int
	Offset int
}

// GenerateMemoirRequest 生成回忆录请求
type GenerateMemoirRequest struct {
	ConversationID uuid.UUID
	Messages       []MessageForGeneration
}

// MessageForGeneration 用于生成回忆录的消息
type MessageForGeneration struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GeneratedMemoir AI 生成的回忆录结果
type GeneratedMemoir struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	TimePeriod string `json:"time_period"`
	StartYear  *int   `json:"start_year"`
	EndYear    *int   `json:"end_year"`
}
