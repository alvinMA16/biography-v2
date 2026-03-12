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

func TestExtractTopicYearRange(t *testing.T) {
	context := "关联时期：返乡创业｜2019-2022\n主线目标：从离职到重新开始的转折\n可展开脉络：1. 离职原因 2. 创业初期 3. 后来的变化\n不要陷入：不要只盯着某个产品细节\n抬高一层可问：那时候最让您觉得必须变一变的，是什么？"

	startYear, endYear, ok := extractTopicYearRange(context)
	if !ok {
		t.Fatal("expected year range to be extracted")
	}
	if startYear != 2019 || endYear != 2022 {
		t.Fatalf("unexpected year range: %d-%d", startYear, endYear)
	}
}

func TestTopicBatchYearWindow(t *testing.T) {
	topics := []domainTopic.GeneratedTopic{
		{
			Context: "关联时期：求学｜1977-1981\n主线目标：恢复高考给家庭带来的改变\n可展开脉络：1. 备考 2. 家里支持 3. 离家上学\n不要陷入：不要一直问某一道题\n抬高一层可问：那几年最改变您命运的，是哪一步？",
		},
		{
			Context: "关联时期：进城打工｜1998-2003\n主线目标：第一次真正独立生活\n可展开脉络：1. 为什么离开家 2. 刚到城市的适应 3. 后来怎么站稳脚跟\n不要陷入：不要一直追着某件行李问\n抬高一层可问：您觉得那次离开，真正改变了您什么？",
		},
	}

	startYear, endYear, ok := topicBatchYearWindow(topics)
	if !ok {
		t.Fatal("expected batch year window")
	}
	if startYear != 1977 || endYear != 2003 {
		t.Fatalf("unexpected batch year window: %d-%d", startYear, endYear)
	}
}
