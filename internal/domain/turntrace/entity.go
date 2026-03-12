package turntrace

import (
	"time"

	"github.com/google/uuid"
)

type Outcome string

const (
	OutcomeCompleted  Outcome = "completed"
	OutcomeEmptyInput Outcome = "empty_input"
	OutcomeLLMError   Outcome = "llm_error"
	OutcomeTTSError   Outcome = "tts_error"
)

type UserTextSource string

const (
	UserTextSourceFinal   UserTextSource = "final"
	UserTextSourceInterim UserTextSource = "interim"
	UserTextSourceEmpty   UserTextSource = "empty"
)

// TurnDiagnostic 记录一轮实时对话的关键阶段时间点。
type TurnDiagnostic struct {
	ID               uuid.UUID      `json:"id" db:"id"`
	ConversationID   uuid.UUID      `json:"conversation_id" db:"conversation_id"`
	UserID           uuid.UUID      `json:"user_id" db:"user_id"`
	TurnIndex        int            `json:"turn_index" db:"turn_index"`
	Mode             string         `json:"mode" db:"mode"`
	Outcome          Outcome        `json:"outcome" db:"outcome"`
	UserTextSource   UserTextSource `json:"user_text_source" db:"user_text_source"`
	UserTextPreview  string         `json:"user_text_preview" db:"user_text_preview"`
	UserTextLength   int            `json:"user_text_length" db:"user_text_length"`
	AssistantPreview string         `json:"assistant_preview" db:"assistant_preview"`
	AssistantLength  int            `json:"assistant_length" db:"assistant_length"`
	UserStopAt       time.Time      `json:"user_stop_at" db:"user_stop_at"`
	ASRFinalAt       *time.Time     `json:"asr_final_at,omitempty" db:"asr_final_at"`
	LLMStartedAt     *time.Time     `json:"llm_started_at,omitempty" db:"llm_started_at"`
	LLMCompletedAt   *time.Time     `json:"llm_completed_at,omitempty" db:"llm_completed_at"`
	TTSStartedAt     *time.Time     `json:"tts_started_at,omitempty" db:"tts_started_at"`
	TTSFirstChunkAt  *time.Time     `json:"tts_first_chunk_at,omitempty" db:"tts_first_chunk_at"`
	TTSCompletedAt   *time.Time     `json:"tts_completed_at,omitempty" db:"tts_completed_at"`
	DoneSentAt       *time.Time     `json:"done_sent_at,omitempty" db:"done_sent_at"`
	ErrorStage       *string        `json:"error_stage,omitempty" db:"error_stage"`
	ErrorMessage     *string        `json:"error_message,omitempty" db:"error_message"`
	CreatedAt        time.Time      `json:"created_at" db:"created_at"`
}

type CreateInput struct {
	ConversationID   uuid.UUID
	UserID           uuid.UUID
	TurnIndex        int
	Mode             string
	Outcome          Outcome
	UserTextSource   UserTextSource
	UserTextPreview  string
	UserTextLength   int
	AssistantPreview string
	AssistantLength  int
	UserStopAt       time.Time
	ASRFinalAt       *time.Time
	LLMStartedAt     *time.Time
	LLMCompletedAt   *time.Time
	TTSStartedAt     *time.Time
	TTSFirstChunkAt  *time.Time
	TTSCompletedAt   *time.Time
	DoneSentAt       *time.Time
	ErrorStage       *string
	ErrorMessage     *string
}
