package preset

import (
	"time"

	"github.com/google/uuid"
)

// Topic 预设话题（全局共享，新用户首次对话时使用）
type Topic struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Topic       string    `json:"topic" db:"topic"`
	Greeting    string    `json:"greeting" db:"greeting"`
	ChatContext *string   `json:"chat_context" db:"chat_context"`
	AgeStart    *int      `json:"age_start" db:"age_start"`
	AgeEnd      *int      `json:"age_end" db:"age_end"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	SortOrder   int       `json:"sort_order" db:"sort_order"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// CreateTopicInput 创建预设话题输入
type CreateTopicInput struct {
	Topic       string `json:"topic" binding:"required"`
	Greeting    string `json:"greeting" binding:"required"`
	ChatContext string `json:"chat_context"`
	AgeStart    *int   `json:"age_start"`
	AgeEnd      *int   `json:"age_end"`
	IsActive    *bool  `json:"is_active"`
	SortOrder   *int   `json:"sort_order"`
}

// UpdateTopicInput 更新预设话题输入
type UpdateTopicInput struct {
	Topic       *string `json:"topic"`
	Greeting    *string `json:"greeting"`
	ChatContext *string `json:"chat_context"`
	AgeStart    *int    `json:"age_start"`
	AgeEnd      *int    `json:"age_end"`
	IsActive    *bool   `json:"is_active"`
	SortOrder   *int    `json:"sort_order"`
}

// ListTopicsFilter 预设话题列表过滤条件
type ListTopicsFilter struct {
	ActiveOnly bool
	Limit      int
	Offset     int
}
