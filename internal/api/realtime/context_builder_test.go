package realtime

import (
	"strings"
	"testing"

	"github.com/peizhengma/biography-v2/internal/provider/llm"
)

func TestBuildTurnContextKeepsLongUserTurnRaw(t *testing.T) {
	longText := strings.Repeat("1968年我们全家从哈尔滨搬到大庆，父亲因为工作调动去了厂里，我心里一直舍不得老家。", 8)

	turn := buildTurnContext(llm.Message{
		Role:    "user",
		Content: longText,
	})

	if turn.IsCompressed {
		t.Fatalf("expected long user turn not to be compressed")
	}
	if turn.WorkingText != longText {
		t.Fatalf("expected long turn working text to stay raw, got %q", turn.WorkingText)
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

func TestBuildChatContextPacketKeepsCurrentUserTurnRaw(t *testing.T) {
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
	if packet.CurrentUserTurn.IsCompressed {
		t.Fatalf("expected current user turn to stay raw")
	}
	if packet.CurrentUserTurn.WorkingText != longText {
		t.Fatalf("expected current user turn working text to stay raw, got %q", packet.CurrentUserTurn.WorkingText)
	}
	if len(packet.RecentTurns) != 1 {
		t.Fatalf("expected 1 recent turn, got %d", len(packet.RecentTurns))
	}
	if packet.RecentTurns[0].WorkingText == "" {
		t.Fatalf("expected recent turn working text not to be empty")
	}
}

func TestBuildChatContextPacketKeepsFullTopicContext(t *testing.T) {
	longContext := "关联时期：重返职场｜2021-2023\n主线目标：重新回到工作状态背后的生活转折\n可展开脉络：1. 离职后的生活节奏 2. 做决定前的顾虑 3. 回到工作后的变化\n不要陷入：不要一直盯着某一轮面试或者某辆车的细节\n抬高一层可问：那时候最让您觉得必须重新出发的，是什么？"
	config := &SessionConfig{
		TopicTitle:   "决定重回职场的那一刻",
		TopicContext: longContext,
		UserName:     "张先生",
	}

	packet := buildChatContextPacket(config, []llm.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "后来我还是决定重新找工作。"},
	})

	if packet.Topic.Context != longContext {
		t.Fatalf("expected full topic context to be preserved, got %q", packet.Topic.Context)
	}
}
