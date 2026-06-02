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

func TestSessionDropsOldestMessagesWhenBudgetExceeded(t *testing.T) {
	recent := openai.Message{Role: "assistant", Content: "recent answer"}
	session := New(messageBytes([]openai.Message{recent}) + 1)
	session.Append(
		openai.Message{Role: "user", Content: "old request that should be dropped"},
		recent,
	)
	history := session.History()
	if len(history) != 1 || history[0].Content != recent.Content {
		t.Fatalf("history = %#v", history)
	}
}

func TestTrimMessagesKeepsToolCallAndResultsTogether(t *testing.T) {
	toolCall := openai.Message{
		Role: "assistant",
		ToolCalls: []openai.ToolCall{{
			ID: "1", Type: "function", Function: openai.FunctionCall{Name: "readFile", Arguments: `{"path":"note.md"}`},
		}},
	}
	toolResult := openai.Message{Role: "tool", ToolCallID: "1", Content: `{"content":"important"}`}
	recent := openai.Message{Role: "assistant", Content: "recent answer"}
	kept := []openai.Message{toolCall, toolResult, recent}
	trimmed, dropped := TrimMessages(append([]openai.Message{{Role: "user", Content: "old request"}}, kept...), messageBytes(kept))
	if dropped != 1 || len(trimmed) != len(kept) || trimmed[0].ToolCalls[0].ID != "1" || trimmed[1].ToolCallID != "1" {
		t.Fatalf("trimmed = %#v, dropped = %d", trimmed, dropped)
	}
}

func TestTrimMessagesKeepsNewestMessageEvenWhenOversized(t *testing.T) {
	trimmed, dropped := TrimMessages([]openai.Message{
		{Role: "user", Content: "old"},
		{Role: "assistant", Content: "newest but larger than budget"},
	}, 1)
	if dropped != 1 || len(trimmed) != 1 || trimmed[0].Content != "newest but larger than budget" {
		t.Fatalf("trimmed = %#v, dropped = %d", trimmed, dropped)
	}
}
