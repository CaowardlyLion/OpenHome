package session

import (
	"testing"

	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
)

func TestSessionPreservesNativeMessagesAndClears(t *testing.T) {
	var session Session
	session.Append(
		openai.Message{Role: "assistant", ToolCalls: []openai.ToolCall{{ID: "1", Type: "function"}}},
		openai.Message{Role: "tool", ToolCallID: "1", Content: `{"written":true}`},
	)
	history := session.History()
	if len(history) != 2 || history[1].Role != "tool" {
		t.Fatalf("history = %#v", history)
	}
	session.Clear()
	if len(session.History()) != 0 {
		t.Fatal("clear retained messages")
	}
}
