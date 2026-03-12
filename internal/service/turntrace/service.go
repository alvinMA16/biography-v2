package turntrace

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/peizhengma/biography-v2/internal/domain/turntrace"
	traceRepo "github.com/peizhengma/biography-v2/internal/repository/turntrace"
)

const previewMaxRunes = 240

type Service struct {
	repo *traceRepo.Repository
}

func New(repo *traceRepo.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input *turntrace.CreateInput) (*turntrace.TurnDiagnostic, error) {
	item := &turntrace.TurnDiagnostic{
		ID:               uuid.New(),
		ConversationID:   input.ConversationID,
		UserID:           input.UserID,
		TurnIndex:        input.TurnIndex,
		Mode:             input.Mode,
		Outcome:          input.Outcome,
		UserTextSource:   input.UserTextSource,
		UserTextPreview:  truncateRunes(input.UserTextPreview, previewMaxRunes),
		UserTextLength:   input.UserTextLength,
		AssistantPreview: truncateRunes(input.AssistantPreview, previewMaxRunes),
		AssistantLength:  input.AssistantLength,
		UserStopAt:       input.UserStopAt,
		ASRFinalAt:       input.ASRFinalAt,
		LLMStartedAt:     input.LLMStartedAt,
		LLMCompletedAt:   input.LLMCompletedAt,
		TTSStartedAt:     input.TTSStartedAt,
		TTSFirstChunkAt:  input.TTSFirstChunkAt,
		TTSCompletedAt:   input.TTSCompletedAt,
		DoneSentAt:       input.DoneSentAt,
		ErrorStage:       nilIfBlank(input.ErrorStage),
		ErrorMessage:     nilIfBlank(input.ErrorMessage),
		CreatedAt:        input.UserStopAt,
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}

	if err := s.repo.CleanupExpired(ctx); err != nil {
		log.Printf("[TurnTrace] 清理过期诊断记录失败: %v", err)
	}

	return item, nil
}

func (s *Service) ListByConversationID(ctx context.Context, conversationID uuid.UUID, limit int) ([]*turntrace.TurnDiagnostic, error) {
	return s.repo.ListByConversationID(ctx, conversationID, limit)
}

func nilIfBlank(value *string) *string {
	if value == nil {
		return nil
	}
	if *value == "" {
		return nil
	}
	return value
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}

	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
