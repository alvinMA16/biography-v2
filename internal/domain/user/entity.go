package user

import (
	"time"

	"github.com/google/uuid"
)

// User 用户实体
type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Phone        string     `json:"phone" db:"phone"`
	PasswordHash string     `json:"-" db:"password_hash"`

	// 基本信息
	Nickname      *string `json:"nickname" db:"nickname"`
	PreferredName *string `json:"preferred_name" db:"preferred_name"`
	Gender        *string `json:"gender" db:"gender"`
	BirthYear     *int    `json:"birth_year" db:"birth_year"`
	Hometown      *string `json:"hometown" db:"hometown"`
	MainCity      *string `json:"main_city" db:"main_city"`

	// 状态
	ProfileCompleted  bool    `json:"profile_completed" db:"profile_completed"`
	EraMemories       *string `json:"era_memories" db:"era_memories"`
	EraMemoriesStatus string  `json:"era_memories_status" db:"era_memories_status"` // none/pending/generating/completed/failed
	IsAdmin           bool    `json:"is_admin" db:"is_admin"`
	IsActive          bool    `json:"is_active" db:"is_active"`

	// 时间戳
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt *time.Time `json:"-" db:"deleted_at"`
}

// DisplayName 返回用于显示的名称（优先称呼，其次姓名）
func (u *User) DisplayName() string {
	if u.PreferredName != nil && *u.PreferredName != "" {
		return *u.PreferredName
	}
	if u.Nickname != nil {
		return *u.Nickname
	}
	return ""
}

// CreateUserInput 创建用户输入
type CreateUserInput struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Nickname string `json:"nickname"`
}

// UpdateProfileInput 更新用户资料输入
type UpdateProfileInput struct {
	Nickname      *string `json:"nickname"`
	PreferredName *string `json:"preferred_name"`
	Gender        *string `json:"gender"`
	BirthYear     *int    `json:"birth_year"`
	Hometown      *string `json:"hometown"`
	MainCity      *string `json:"main_city"`
}

// LoginInput 登录输入
type LoginInput struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse 认证响应
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// AdminUpdateInput 管理员更新用户输入
type AdminUpdateInput struct {
	Nickname         *string `json:"nickname"`
	PreferredName    *string `json:"preferred_name"`
	Gender           *string `json:"gender"`
	BirthYear        *int    `json:"birth_year"`
	Hometown         *string `json:"hometown"`
	MainCity         *string `json:"main_city"`
	ProfileCompleted *bool   `json:"profile_completed"`
	IsActive         *bool   `json:"is_active"`
}

// EraMemoriesStatus 时代记忆状态常量
const (
	EraMemoriesStatusNone       = "none"       // 未收集基础信息
	EraMemoriesStatusPending    = "pending"    // 等待生成
	EraMemoriesStatusGenerating = "generating" // 生成中
	EraMemoriesStatusCompleted  = "completed"  // 已完成
	EraMemoriesStatusFailed     = "failed"     // 失败
)

// AdminCreateInput 管理员创建用户输入
type AdminCreateInput struct {
	Phone    string  `json:"phone" binding:"required"`
	Password string  `json:"password" binding:"required,min=6"`
	Nickname *string `json:"nickname"`
}
