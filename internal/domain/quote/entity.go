package quote

import (
	"time"

	"github.com/google/uuid"
)

// Type 激励语类型
type Type string

const (
	TypeMotivational Type = "motivational" // 激励语
	TypeGreeting     Type = "greeting"     // 问候语
)

// Quote 激励语/问候语实体
type Quote struct {
	ID      uuid.UUID `json:"id" db:"id"`
	Content string    `json:"content" db:"content"`
	Type    Type      `json:"type" db:"type"`

	// 配置
	IsActive     bool `json:"is_active" db:"is_active"`
	ShowGreeting bool `json:"show_greeting" db:"show_greeting"`

	// 时间戳
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CreateQuoteInput 创建激励语输入
type CreateQuoteInput struct {
	Content      string `json:"content" binding:"required"`
	Type         Type   `json:"type" binding:"required"`
	IsActive     *bool  `json:"is_active"`
	ShowGreeting *bool  `json:"show_greeting"`
}

// UpdateQuoteInput 更新激励语输入
type UpdateQuoteInput struct {
	Content      *string `json:"content"`
	Type         *Type   `json:"type"`
	IsActive     *bool   `json:"is_active"`
	ShowGreeting *bool   `json:"show_greeting"`
}
