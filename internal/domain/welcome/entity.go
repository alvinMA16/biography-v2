package welcome

import (
	"time"

	"github.com/google/uuid"
)

// Message 欢迎语实体
type Message struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Content      string    `json:"content" db:"content"`
	ShowGreeting bool      `json:"show_greeting" db:"show_greeting"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	SortOrder    int       `json:"sort_order" db:"sort_order"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// CreateInput 创建欢迎语输入
type CreateInput struct {
	Content      string `json:"content" binding:"required"`
	ShowGreeting *bool  `json:"show_greeting"`
	IsActive     *bool  `json:"is_active"`
	SortOrder    *int   `json:"sort_order"`
}

// UpdateInput 更新欢迎语输入
type UpdateInput struct {
	Content      *string `json:"content"`
	ShowGreeting *bool   `json:"show_greeting"`
	IsActive     *bool   `json:"is_active"`
	SortOrder    *int    `json:"sort_order"`
}
