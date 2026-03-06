package audit

import (
	"time"

	"github.com/google/uuid"
)

// Action 审计操作类型
type Action string

const (
	ActionCreateUser    Action = "create_user"
	ActionEditUser      Action = "edit_user"
	ActionResetPassword Action = "reset_password"
	ActionDeleteUser    Action = "delete_user"
	ActionToggleActive  Action = "toggle_active"
	ActionCreateTopic   Action = "create_topic"
	ActionEditTopic     Action = "edit_topic"
	ActionDeleteTopic   Action = "delete_topic"
	ActionCreateQuote   Action = "create_quote"
	ActionEditQuote     Action = "edit_quote"
	ActionDeleteQuote   Action = "delete_quote"
	ActionCreateWelcome Action = "create_welcome"
	ActionEditWelcome   Action = "edit_welcome"
	ActionDeleteWelcome    Action = "delete_welcome"
	ActionDeleteMemoir     Action = "delete_memoir"
	ActionEditMemoir       Action = "edit_memoir"
	ActionCreateEraMemory  Action = "create_era_memory"
	ActionUpdateEraMemory  Action = "update_era_memory"
	ActionDeleteEraMemory  Action = "delete_era_memory"
	ActionCreatePresetTopic Action = "create_preset_topic"
	ActionUpdatePresetTopic Action = "update_preset_topic"
	ActionDeletePresetTopic Action = "delete_preset_topic"
)

// TargetType 目标类型
type TargetType string

const (
	TargetTypeUser         TargetType = "user"
	TargetTypeConversation TargetType = "conversation"
	TargetTypeMemoir       TargetType = "memoir"
	TargetTypeTopic        TargetType = "topic"
	TargetTypeQuote        TargetType = "quote"
	TargetTypeWelcome      TargetType = "welcome"
	TargetTypeEraMemory    TargetType = "era_memory"
	TargetTypePresetTopic  TargetType = "preset_topic"
)

// Log 审计日志实体
type Log struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	AdminID     *uuid.UUID     `json:"admin_id" db:"admin_id"`
	Action      Action         `json:"action" db:"action"`
	TargetType  TargetType     `json:"target_type" db:"target_type"`
	TargetID    *uuid.UUID     `json:"target_id" db:"target_id"`
	TargetLabel *string        `json:"target_label" db:"target_label"`
	Detail      map[string]any `json:"detail" db:"detail"`
	IPAddress   *string        `json:"ip_address" db:"ip_address"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
}

// CreateInput 创建审计日志输入
type CreateInput struct {
	AdminID     *uuid.UUID
	Action      Action
	TargetType  TargetType
	TargetID    *uuid.UUID
	TargetLabel string
	Detail      map[string]any
	IPAddress   string
}

// ListFilter 审计日志列表过滤条件
type ListFilter struct {
	AdminID    *uuid.UUID
	Action     *Action
	TargetType *TargetType
	TargetID   *uuid.UUID
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	Offset     int
}
