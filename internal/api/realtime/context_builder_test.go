package realtime

import (
	"strings"
	"testing"

	"github.com/peizhengma/biography-v2/internal/provider/llm"
)

func TestBuildTurnContextCompressesLongUserTurn(t *testing.T) {
	longText := strings.Repeat("1968年我们全家从哈尔滨搬到大庆，父亲因为工作调动去了厂里，我心里一直舍不得老家。", 8)

	turn := buildTurnContext(llm.Message{
		Role:    "user",
		Content: longText,
	})

	if !turn.IsCompressed {
		t.Fatalf("expected long user turn to be compressed")
	}
	if !strings.Contains(turn.WorkingText, "最后落点：") {
		t.Fatalf("expected working turn to contain 最后落点, got %q", turn.WorkingText)
	}
	if !strings.Contains(turn.WorkingText, "主线：") {
		t.Fatalf("expected working turn to contain 主线, got %q", turn.WorkingText)
	}
	if !strings.Contains(turn.WorkingText, "关键信息：") {
		t.Fatalf("expected working turn to contain 关键信息, got %q", turn.WorkingText)
	}
	if !strings.Contains(turn.WorkingText, "可追问点：") {
		t.Fatalf("expected working turn to contain 可追问点, got %q", turn.WorkingText)
	}
}

func TestBuildTurnContextKeepsShortUserTurnRaw(t *testing.T) {
	shortText := "后来我们搬到大庆了。"

	turn := buildTurnContext(llm.Message{
		Role:    "user",
		Content: shortText,
	})

	if turn.IsCompressed {
		t.Fatalf("expected short user turn not to be compressed")
	}
	if turn.WorkingText != shortText {
		t.Fatalf("expected working text to equal raw text, got %q", turn.WorkingText)
	}
}

func TestBuildChatContextPacketUsesWorkingTurnForCurrentUserTurn(t *testing.T) {
	config := &SessionConfig{
		TopicTitle:   "第一次离开老家",
		TopicContext: "聊聊离乡时的经历",
		UserName:     "张先生",
	}
	longText := strings.Repeat("1978年我离开老家去外地工作，心里特别舍不得家里人，后来到了新地方一直不太适应。", 12)

	packet := buildChatContextPacket(config, []llm.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "assistant", Content: "您还记得当时是因为什么离开老家的吗？"},
		{Role: "user", Content: longText},
	})

	if packet.CurrentUserTurn.Role != "user" {
		t.Fatalf("expected current turn role to be user, got %s", packet.CurrentUserTurn.Role)
	}
	if !packet.CurrentUserTurn.IsCompressed {
		t.Fatalf("expected current user turn to be compressed")
	}
	if len(packet.RecentTurns) != 1 {
		t.Fatalf("expected 1 recent turn, got %d", len(packet.RecentTurns))
	}
	if packet.RecentTurns[0].WorkingText == "" {
		t.Fatalf("expected recent turn working text not to be empty")
	}
}
