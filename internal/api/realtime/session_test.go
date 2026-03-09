package realtime

import "testing"

func TestEnsureFirstSessionClosingText(t *testing.T) {
	text := ensureFirstSessionClosingText("", true)
	if text == "" {
		t.Fatal("expected default closing text when conversation should end")
	}
}

func TestDecodeAssistantEnvelopeText(t *testing.T) {
	text, shouldEnd := decodeAssistantEnvelope(`{"type":"text","content":"您还记得当时院子里最常玩的是什么吗？"}`, false)
	if shouldEnd {
		t.Fatal("expected text envelope not to end conversation")
	}
	if text == "" {
		t.Fatal("expected text content")
	}
}

func TestDecodeAssistantEnvelopeEndTool(t *testing.T) {
	text, shouldEnd := decodeAssistantEnvelope(`{"type":"tool","tool":"end_conversation","content":"今天先聊到这儿，以后咱们再慢慢聊您的人生故事。"}`, true)
	if !shouldEnd {
		t.Fatal("expected tool envelope to end conversation")
	}
	if text == "" {
		t.Fatal("expected closing text")
	}
}

func TestDecodeAssistantEnvelopeCodeFence(t *testing.T) {
	text, shouldEnd := decodeAssistantEnvelope("```json\n{\"type\":\"text\",\"content\":\"那会儿院子里最热闹的时候一般是什么时候？\"}\n```", false)
	if shouldEnd {
		t.Fatal("expected fenced text envelope not to end conversation")
	}
	if text == "" {
		t.Fatal("expected fenced JSON to parse")
	}
}
