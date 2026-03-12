package flow

import (
	"testing"
	"time"

	"github.com/google/uuid"
	domainTopic "github.com/peizhengma/biography-v2/internal/domain/topic"
)

func TestExtractConversationExcerpt(t *testing.T) {
	conversationText := "用户：先说第一段。\n\n用户：这里是创业压力和找工作的内容，这一段会继续展开，包括员工工资、重新找工作的过程，还有很多具体的细节，比如现金流压力、团队稳定、面试时怎么讲创业经历、重新适应大公司节奏。\n\n用户：最后收在新公司老板这里。"

	excerpt := extractConversationExcerpt(conversationText, "创业压力", "新公司老板")
	if excerpt == "" {
		t.Fatal("expected excerpt, got empty string")
	}
	if excerpt == conversationText {
		t.Fatal("expected narrowed excerpt, got full conversation")
	}
}

func TestExtractConversationExcerptFallsBackWhenTooShort(t *testing.T) {
	conversationText := "用户：这是一个很长的完整对话内容，里面有很多信息，足够长，不应该因为锚点太短而截丢。"

	excerpt := extractConversationExcerpt(conversationText, "很长", "完整")
	if excerpt != conversationText {
		t.Fatal("expected fallback to full conversation for too-short excerpt")
	}
}

func TestFilterTopicCandidatesExcludingIDs(t *testing.T) {
	keepID := uuid.New()
	dropID := uuid.New()
	topics := []*domainTopic.TopicCandidate{
		{ID: keepID},
		{ID: dropID},
	}

	filtered := filterTopicCandidatesExcludingIDs(topics, map[uuid.UUID]struct{}{
		dropID: {},
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 topic after filter, got %d", len(filtered))
	}
	if filtered[0].ID != keepID {
		t.Fatalf("expected topic %s to remain, got %s", keepID, filtered[0].ID)
	}
}

func TestTopicCandidatesToTrimKeepsNewestTopics(t *testing.T) {
	olderID := uuid.New()
	oldID := uuid.New()
	newID := uuid.New()
	now := time.Now()
	topics := []*domainTopic.TopicCandidate{
		{ID: newID, CreatedAt: now},
		{ID: oldID, CreatedAt: now.Add(-time.Minute)},
		{ID: olderID, CreatedAt: now.Add(-2 * time.Minute)},
	}

	trimmed := topicCandidatesToTrim(topics, 2)

	if len(trimmed) != 1 {
		t.Fatalf("expected 1 trimmed topic, got %d", len(trimmed))
	}
	if trimmed[0] != olderID {
		t.Fatalf("expected oldest topic to be trimmed, got %s", trimmed[0])
	}
}

func TestLimitTopicCandidatesReturnsRequestedBatch(t *testing.T) {
	firstID := uuid.New()
	secondID := uuid.New()
	thirdID := uuid.New()
	topics := []*domainTopic.TopicCandidate{
		{ID: firstID},
		{ID: secondID},
		{ID: thirdID},
	}

	limited := limitTopicCandidates(topics, 2)

	if len(limited) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(limited))
	}
	if limited[0].ID != firstID || limited[1].ID != secondID {
		t.Fatalf("expected first two topics to be preserved in order")
	}
}
