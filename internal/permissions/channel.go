package permissions

import "context"

type Pending struct {
	Request  Request
	Response chan Decision
}

type ChannelPrompter struct {
	Requests chan Pending
}

func NewChannelPrompter() *ChannelPrompter {
	return &ChannelPrompter{Requests: make(chan Pending)}
}

func (p *ChannelPrompter) Prompt(ctx context.Context, request Request) Decision {
	pending := Pending{Request: request, Response: make(chan Decision, 1)}
	select {
	case p.Requests <- pending:
	case <-ctx.Done():
		return Deny
	}
	select {
	case decision := <-pending.Response:
		return decision
	case <-ctx.Done():
		return Deny
	}
}
