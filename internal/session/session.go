package session

import (
	"encoding/json"

	"github.com/CaowardlyLion/OpenHome/internal/providers/openai"
)

type Session struct {
	history []openai.Message
}

func (s *Session) History() []openai.Message {
	return append([]openai.Message(nil), s.history...)
}

func (s *Session) Append(messages ...openai.Message) {
	s.history = append(s.history, messages...)
}

func (s *Session) AppendEvent(eventType string, data any) {
	content, _ := json.Marshal(data)
	s.Append(openai.Message{Role: "assistant", Content: "[execution:" + eventType + "] " + string(content)})
}

func (s *Session) Clear() {
	s.history = nil
}
