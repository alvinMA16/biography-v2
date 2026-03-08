package topic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	domainTopic "github.com/peizhengma/biography-v2/internal/domain/topic"
)

func TestGeneratedTopicToCandidatePreservesEraContext(t *testing.T) {
	userID := uuid.New()
	now := time.Now()

	candidate := generatedTopicToCandidate(userID, domainTopic.GeneratedTopic{
		Title:      "第一次参加工作",
		Greeting:   "咱们聊聊您刚参加工作的时候吧。",
		Context:    "从第一份工作怎么开始讲起。",
		EraContext: "这段经历常和单位分配、进厂、离开老家有关。",
	}, now)

	if candidate.UserID != userID {
		t.Fatalf("expected user id to be preserved")
	}
	if candidate.EraContext == nil || *candidate.EraContext == "" {
		t.Fatalf("expected era context to be preserved")
	}
	if *candidate.EraContext != "这段经历常和单位分配、进厂、离开老家有关。" {
		t.Fatalf("unexpected era context: %q", *candidate.EraContext)
	}
	if candidate.Status != domainTopic.StatusPending {
		t.Fatalf("expected status pending, got %s", candidate.Status)
	}
}

func TestCandidateToOptionIncludesEraContext(t *testing.T) {
	eraContext := "这类话题常带有离乡和适应新环境的情绪。"
	candidate := &domainTopic.TopicCandidate{
		ID:         uuid.New(),
		Title:      "离开老家那几年",
		Greeting:   strPtr("咱们聊聊您第一次离开老家的时候。"),
		Context:    strPtr("先从为什么离开讲起。"),
		EraContext: &eraContext,
	}

	option := candidateToOption(candidate)

	if option.EraContext != eraContext {
		t.Fatalf("expected era context to be mapped, got %q", option.EraContext)
	}
	if option.Context != "先从为什么离开讲起。" {
		t.Fatalf("expected context to be mapped, got %q", option.Context)
	}
}

func strPtr(s string) *string {
	return &s
}
