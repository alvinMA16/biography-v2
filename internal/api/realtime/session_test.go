package realtime

import "testing"

func TestEnsureProfileCollectionClosingText(t *testing.T) {
	text := ensureProfileCollectionClosingText("", true)
	if text == "" {
		t.Fatal("expected default closing text when conversation should end")
	}
}
