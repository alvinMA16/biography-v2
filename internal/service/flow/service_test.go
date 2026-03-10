package flow

import "testing"

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
