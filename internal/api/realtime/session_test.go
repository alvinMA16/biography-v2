package realtime

import "testing"

func TestEnsureFirstSessionClosingText(t *testing.T) {
	text := ensureFirstSessionClosingText("", true)
	if text == "" {
		t.Fatal("expected default closing text when conversation should end")
	}
}
