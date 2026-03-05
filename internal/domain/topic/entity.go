package topic

import (
	"time"

	"github.com/google/uuid"
)

// Status 话题状态
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusUsed     Status = "used"
)

// Source 话题来源
type Source string

const (
	SourceAI     Source = "ai"
	SourceManual Source = "manual"
)

// TopicCandidate 话题候选实体
type TopicCandidate struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"user_id" db:"user_id"`

	// 话题内容
	Title    string  `json:"title" db:"title"`
	Greeting *string `json:"greeting" db:"greeting"`
	Context  *string `json:"context" db:"context"`

	// 状态
	Status Status `json:"status" db:"status"`
	Source Source `json:"source" db:"source"`

	// 时间戳
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TopicOption 话题选项（返回给前端）
type TopicOption struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	Greeting string    `json:"greeting"`
	Context  string    `json:"context"`
}

// CreateTopicInput 创建话题输入
type CreateTopicInput struct {
	Title    string  `json:"title" binding:"required"`
	Greeting string  `json:"greeting"`
	Context  string  `json:"context"`
	Source   Source  `json:"source"`
	Status   *Status `json:"status"` // 管理员可指定初始状态
}

// UpdateTopicInput 更新话题输入
type UpdateTopicInput struct {
	Title    *string `json:"title"`
	Greeting *string `json:"greeting"`
	Context  *string `json:"context"`
	Status   *Status `json:"status"`
}

// GenerateTopicsRequest 生成话题请求
type GenerateTopicsRequest struct {
	UserID         uuid.UUID
	UserProfile    UserProfileForTopic
	ExistingTopics []string
	ExistingMemoir []string
	Count          int
}

// UserProfileForTopic 用于生成话题的用户信息
type UserProfileForTopic struct {
	Nickname  string
	BirthYear *int
	Hometown  string
	MainCity  string
}

// GeneratedTopic AI 生成的话题
type GeneratedTopic struct {
	Title    string `json:"title"`
	Greeting string `json:"greeting"`
	Context  string `json:"context"`
}
