package turntrace

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peizhengma/biography-v2/internal/domain/turntrace"
)

const retentionPeriod = 14 * 24 * time.Hour

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, item *turntrace.TurnDiagnostic) error {
	query := `
		INSERT INTO conversation_turn_diagnostics (
			id, conversation_id, user_id, turn_index, mode, outcome, user_text_source,
			user_text_preview, user_text_length, assistant_preview, assistant_length,
			user_stop_at, asr_final_at, llm_started_at, llm_completed_at,
			tts_started_at, tts_first_chunk_at, tts_completed_at, done_sent_at,
			error_stage, error_message, created_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22
		)
	`

	_, err := r.pool.Exec(ctx, query,
		item.ID,
		item.ConversationID,
		item.UserID,
		item.TurnIndex,
		item.Mode,
		item.Outcome,
		item.UserTextSource,
		item.UserTextPreview,
		item.UserTextLength,
		item.AssistantPreview,
		item.AssistantLength,
		item.UserStopAt,
		item.ASRFinalAt,
		item.LLMStartedAt,
		item.LLMCompletedAt,
		item.TTSStartedAt,
		item.TTSFirstChunkAt,
		item.TTSCompletedAt,
		item.DoneSentAt,
		item.ErrorStage,
		item.ErrorMessage,
		item.CreatedAt,
	)
	return err
}

func (r *Repository) ListByConversationID(ctx context.Context, conversationID uuid.UUID, limit int) ([]*turntrace.TurnDiagnostic, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, conversation_id, user_id, turn_index, mode, outcome, user_text_source,
		       user_text_preview, user_text_length, assistant_preview, assistant_length,
		       user_stop_at, asr_final_at, llm_started_at, llm_completed_at,
		       tts_started_at, tts_first_chunk_at, tts_completed_at, done_sent_at,
		       error_stage, error_message, created_at
		FROM conversation_turn_diagnostics
		WHERE conversation_id = $1
		ORDER BY turn_index ASC, created_at ASC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*turntrace.TurnDiagnostic
	for rows.Next() {
		var item turntrace.TurnDiagnostic
		if err := rows.Scan(
			&item.ID,
			&item.ConversationID,
			&item.UserID,
			&item.TurnIndex,
			&item.Mode,
			&item.Outcome,
			&item.UserTextSource,
			&item.UserTextPreview,
			&item.UserTextLength,
			&item.AssistantPreview,
			&item.AssistantLength,
			&item.UserStopAt,
			&item.ASRFinalAt,
			&item.LLMStartedAt,
			&item.LLMCompletedAt,
			&item.TTSStartedAt,
			&item.TTSFirstChunkAt,
			&item.TTSCompletedAt,
			&item.DoneSentAt,
			&item.ErrorStage,
			&item.ErrorMessage,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}

	return items, nil
}

func (r *Repository) CleanupExpired(ctx context.Context) error {
	query := `DELETE FROM conversation_turn_diagnostics WHERE created_at < $1`
	_, err := r.pool.Exec(ctx, query, time.Now().Add(-retentionPeriod))
	return err
}

func (r *Repository) DeleteByConversationID(ctx context.Context, conversationID uuid.UUID) error {
	query := `DELETE FROM conversation_turn_diagnostics WHERE conversation_id = $1`
	_, err := r.pool.Exec(ctx, query, conversationID)
	return err
}
