package session

import (
	"encoding/json"

	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
)

type Session struct {
	history  []openai.Message
	maxBytes int
}

func New(maxBytes int) *Session {
	return &Session{maxBytes: maxBytes}
}

func (s *Session) History() []openai.Message {
	return append([]openai.Message(nil), s.history...)
}

func (s *Session) Append(messages ...openai.Message) {
	s.history = append(s.history, messages...)
	s.history, _ = TrimMessages(s.history, s.maxBytes)
}

func (s *Session) AppendEvent(eventType string, data any) {
	content, _ := json.Marshal(data)
	s.Append(openai.Message{Role: "assistant", Content: "[execution:" + eventType + "] " + string(content)})
}

func (s *Session) Clear() {
	s.history = nil
}

func TrimMessages(messages []openai.Message, maxBytes int) ([]openai.Message, int) {
	if maxBytes <= 0 || messageBytes(messages) <= maxBytes {
		return append([]openai.Message(nil), messages...), 0
	}
	units := messageUnits(messages)
	total := messageBytes(messages)
	dropped := 0
	for len(units) > 1 && total > maxBytes {
		total -= messageBytes(units[0])
		dropped += len(units[0])
		units = units[1:]
	}
	result := make([]openai.Message, 0, len(messages)-dropped)
	for _, unit := range units {
		result = append(result, unit...)
	}
	return result, dropped
}

func messageUnits(messages []openai.Message) [][]openai.Message {
	var units [][]openai.Message
	for index := 0; index < len(messages); {
		end := index + 1
		if len(messages[index].ToolCalls) > 0 {
			for end < len(messages) && messages[end].Role == "tool" {
				end++
			}
		}
		units = append(units, messages[index:end])
		index = end
	}
	return units
}

func messageBytes(messages []openai.Message) int {
	content, _ := json.Marshal(messages)
	return len(content)
}
