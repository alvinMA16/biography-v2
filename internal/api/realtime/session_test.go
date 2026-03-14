package realtime

import (
	"encoding/base64"
	"testing"
)

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

func TestHandleAudioIgnoredOutsideListening(t *testing.T) {
	session := &Session{}

	if err := session.handleAudio(base64.StdEncoding.EncodeToString([]byte{0x00, 0x01})); err != nil {
		t.Fatalf("expected audio outside listening to be ignored, got error: %v", err)
	}
	if session.currentTurnASRBuffer != nil {
		t.Fatal("expected no ASR turn buffer to be created outside listening")
	}
}

func TestTurnASRBufferIsolationAcrossTurns(t *testing.T) {
	session := &Session{}
	session.setState(StateListening, "test")

	firstTurn := session.ensureCurrentTurnASRBuffer()
	firstTurn.appendResult("第一轮内容。", true)

	finalizing := session.beginTurnASRFinalization()
	if finalizing != firstTurn {
		t.Fatal("expected finalizing buffer to be the first turn buffer")
	}

	secondTurn := session.ensureCurrentTurnASRBuffer()
	if secondTurn == firstTurn {
		t.Fatal("expected a new buffer for the next turn")
	}
	secondTurn.appendResult("第二轮内容。", true)

	firstText, source, _ := session.collectUserTextWithGrace(finalizing)
	if source != "final" {
		t.Fatalf("expected final source, got %s", source)
	}
	if firstText != "第一轮内容。" {
		t.Fatalf("expected first turn text only, got %q", firstText)
	}

	secondText, _, _ := secondTurn.consume()
	if secondText != "第二轮内容。" {
		t.Fatalf("expected second turn text only, got %q", secondText)
	}
}
