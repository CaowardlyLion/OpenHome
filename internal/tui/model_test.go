package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/CaowardlyLion/OpenHome/internal/agents"
)

type fakeHandler struct {
	newSessions int
}

func (f *fakeHandler) Handle(context.Context, string) (agents.Result, error) {
	return agents.Result{Answer: "Done.", LogPath: "run.jsonl"}, nil
}

func (f *fakeHandler) NewSession() {
	f.newSessions++
}

func TestModelStartsTaskAndRendersResult(t *testing.T) {
	handler := &fakeHandler{}
	model := New(handler)
	model.input.SetValue("add milk")
	updated, command := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.busy || command == nil {
		t.Fatalf("busy = %v, command = %v", model.busy, command)
	}
	updated, _ = model.Update(command())
	model = updated.(Model)
	if model.busy || !strings.Contains(model.View().Content, "[assistant] Done.") || !strings.Contains(model.View().Content, "[log] run.jsonl") {
		t.Fatalf("view = %q", model.View().Content)
	}
}

func TestModelDispatchesNewSession(t *testing.T) {
	handler := &fakeHandler{}
	model := New(handler)
	model.input.SetValue("/new")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if handler.newSessions != 1 || !strings.Contains(model.View().Content, "Started new session") {
		t.Fatalf("newSessions = %d, view = %q", handler.newSessions, model.View().Content)
	}
}

func TestModelHidesStatusDetailsByDefault(t *testing.T) {
	model := New(&fakeHandler{})
	updated, _ := model.Update(StatusMsg{Type: "intent", Message: "Add milk."})
	model = updated.(Model)
	updated, _ = model.Update(StatusMsg{Type: "tool", Message: "writeFile"})
	model = updated.(Model)
	view := model.View().Content
	if strings.Contains(view, "[intent] Add milk.") || strings.Contains(view, "[tool] writeFile") || model.status != "writeFile" || !strings.Contains(view, "writeFile") {
		t.Fatalf("status = %q, view = %q", model.status, view)
	}
}

func TestModelTogglesVerboseStatusDetails(t *testing.T) {
	model := New(&fakeHandler{})
	model.input.SetValue("/verbose")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if !model.verbose || !strings.Contains(model.View().Content, "Verbose execution details shown.") {
		t.Fatalf("verbose = %v, view = %q", model.verbose, model.View().Content)
	}
	updated, _ = model.Update(StatusMsg{Type: "intent", Message: "Add milk."})
	model = updated.(Model)
	if !strings.Contains(model.View().Content, "[intent] Add milk.") {
		t.Fatalf("view = %q", model.View().Content)
	}
	model.input.SetValue("/verbose")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.verbose || !strings.Contains(model.View().Content, "Verbose execution details hidden.") {
		t.Fatalf("verbose = %v, view = %q", model.verbose, model.View().Content)
	}
}

func TestModelCancelsBusyTask(t *testing.T) {
	model := New(&fakeHandler{})
	cancelled := false
	model.busy = true
	model.cancel = func() { cancelled = true }
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if !cancelled || !strings.Contains(model.status, "Cancelling") {
		t.Fatalf("cancelled = %v, status = %q", cancelled, model.status)
	}
}
