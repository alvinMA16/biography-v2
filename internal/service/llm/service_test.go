package llm

import (
	"testing"

	"github.com/peizhengma/biography-v2/internal/domain/topic"
)

func TestNormalizeMemoirPlans(t *testing.T) {
	plans := []PlannedMemoir{
		{
			ShouldGenerate:  false,
			Theme:           "should be ignored",
			CoverageSummary: "ignored",
		},
		{
			ShouldGenerate:  true,
			TitleHint:       "  第一篇  ",
			Theme:           "  成都最后一晚  ",
			CoverageSummary: "  从出差到自由的感受  ",
		},
		{
			ShouldGenerate:  true,
			TitleHint:       "第二篇",
			Theme:           "",
			CoverageSummary: "缺主题，应该被过滤",
		},
	}

	normalized := normalizeMemoirPlans(plans)
	if len(normalized) != 1 {
		t.Fatalf("expected 1 normalized plan, got %d", len(normalized))
	}
	if normalized[0].TitleHint != "第一篇" {
		t.Fatalf("unexpected title hint: %q", normalized[0].TitleHint)
	}
	if normalized[0].Confidence != "medium" {
		t.Fatalf("expected default confidence medium, got %q", normalized[0].Confidence)
	}
}

func TestNormalizeGeneratedTopics(t *testing.T) {
	topics := []topic.GeneratedTopic{
		{
			Title:      "  决定重回职场的那一刻  ",
			Greeting:   "  咱们聊聊您决定重回职场的时候。  ",
			Context:    "关联时期：重返职场｜2021-2023\r\n\n主线目标：这次决定为什么成了一个转折\r\n可展开脉络：1. 离职后的日子 2. 做决定前的犹豫 3. 回到职场后的变化\r\n不要陷入：不要一直围绕某个面试细节打转\r\n抬高一层可问：那时候最让您下定决心的，其实是什么？\n",
			EraContext: "  ",
		},
		{
			Title:    "缺字段的话题",
			Greeting: "",
			Context:  "关联时期：童年｜1988-1992",
		},
	}

	normalized := normalizeGeneratedTopics(topics)
	if len(normalized) != 1 {
		t.Fatalf("expected 1 normalized topic, got %d", len(normalized))
	}
	if normalized[0].Title != "决定重回职场的那一刻" {
		t.Fatalf("unexpected title: %q", normalized[0].Title)
	}
	if normalized[0].EraContext != "" {
		t.Fatalf("expected empty era context after trim, got %q", normalized[0].EraContext)
	}
	expectedContext := "关联时期：重返职场｜2021-2023\n主线目标：这次决定为什么成了一个转折\n可展开脉络：1. 离职后的日子 2. 做决定前的犹豫 3. 回到职场后的变化\n不要陷入：不要一直围绕某个面试细节打转\n抬高一层可问：那时候最让您下定决心的，其实是什么？"
	if normalized[0].Context != expectedContext {
		t.Fatalf("unexpected normalized context: %q", normalized[0].Context)
	}
}
