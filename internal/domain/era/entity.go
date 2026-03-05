package era

import (
	"time"

	"github.com/google/uuid"
)

// MemoryPreset 时代记忆预设（全局历史事件库）
type MemoryPreset struct {
	ID        uuid.UUID `json:"id" db:"id"`
	StartYear int       `json:"start_year" db:"start_year"`
	EndYear   int       `json:"end_year" db:"end_year"`
	Category  *string   `json:"category" db:"category"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CreateMemoryPresetInput 创建时代记忆预设输入
type CreateMemoryPresetInput struct {
	StartYear int    `json:"start_year" binding:"required"`
	EndYear   int    `json:"end_year" binding:"required"`
	Category  string `json:"category"`
	Content   string `json:"content" binding:"required"`
}

// UpdateMemoryPresetInput 更新时代记忆预设输入
type UpdateMemoryPresetInput struct {
	StartYear *int    `json:"start_year"`
	EndYear   *int    `json:"end_year"`
	Category  *string `json:"category"`
	Content   *string `json:"content"`
}

// ListMemoryPresetsFilter 时代记忆预设列表过滤条件
type ListMemoryPresetsFilter struct {
	Category  *string
	YearStart *int // 筛选覆盖这个年份的记忆
	YearEnd   *int
	Limit     int
	Offset    int
}
