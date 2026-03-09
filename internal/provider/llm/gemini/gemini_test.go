package gemini

import "testing"

func TestParseToolResponse_WithFunctionCallAndText(t *testing.T) {
	resp, err := parseToolResponse(&generateContentResponse{
		Candidates: []struct {
			Content      *geminiContent `json:"content"`
			FinishReason string         `json:"finishReason"`
		}{
			{
				Content: &geminiContent{
					Role: "model",
					Parts: []geminiPart{
						{Text: "今天先聊到这儿。"},
						{FunctionCall: &geminiFunctionCall{
							Name: "end_conversation",
							Args: map[string]interface{}{},
						}},
					},
				},
				FinishReason: "STOP",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseToolResponse returned error: %v", err)
	}
	if resp.Content != "今天先聊到这儿。" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Function.Name != "end_conversation" {
		t.Fatalf("unexpected tool name: %s", resp.ToolCalls[0].Function.Name)
	}
	if resp.ToolCalls[0].Function.Arguments != "{}" {
		t.Fatalf("unexpected tool args: %s", resp.ToolCalls[0].Function.Arguments)
	}
	if resp.FinishReason != "stop" {
		t.Fatalf("unexpected finish reason: %s", resp.FinishReason)
	}
}
