package llm

import "testing"

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
