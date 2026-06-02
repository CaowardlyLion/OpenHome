package permissions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func manager(t *testing.T, mode Mode, prompt PromptFunc) *Manager {
	t.Helper()
	result, err := New(t.TempDir(), t.TempDir(), mode, prompt)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAuthorizeRemembersSimilarInDefaultMode(t *testing.T) {
	calls := 0
	manager := manager(t, Default, func(context.Context, Request) Decision {
		calls++
		return AllowSimilar
	})
	request := Request{Tool: "downloadFile", SimilarKey: "https://example.com/a"}
	if manager.Authorize(context.Background(), request) != AllowSimilar || manager.Authorize(context.Background(), request) != AllowOnce || calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestAskModeStillPromptsForRememberedRule(t *testing.T) {
	calls := 0
	manager := manager(t, Default, func(context.Context, Request) Decision {
		calls++
		return AllowSimilar
	})
	request := Request{Tool: "downloadFile", SimilarKey: "x"}
	manager.Authorize(context.Background(), request)
	if err := manager.SetMode(Ask); err != nil {
		t.Fatal(err)
	}
	manager.Authorize(context.Background(), request)
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestShellRulesSafeElevatedAndDenied(t *testing.T) {
	calls := 0
	manager := manager(t, Allow, func(context.Context, Request) Decision {
		calls++
		return AllowOnce
	})
	if manager.AuthorizeCommand(context.Background(), "ls", []string{"."}, "inspect", 30) != AllowOnce || calls != 0 {
		t.Fatal("safe ls unexpectedly prompted")
	}
	if manager.AuthorizeCommand(context.Background(), "sleep", []string{"1"}, "wait", 60) != AllowOnce || calls != 1 {
		t.Fatal("elevated timeout did not prompt")
	}
	if manager.AuthorizeCommand(context.Background(), "sudo", []string{"ls"}, "inspect", 30) != Deny || calls != 1 {
		t.Fatal("deny rule did not override allow mode")
	}
}

func TestAskModePromptsForSafeInspectionCommand(t *testing.T) {
	calls := 0
	manager := manager(t, Ask, func(context.Context, Request) Decision {
		calls++
		return Deny
	})
	if manager.AuthorizeCommand(context.Background(), "ls", []string{"."}, "inspect", 30) != Deny || calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestInvalidRegexFailsStartup(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "shell-allow.txt"), []byte("[invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDir, t.TempDir(), Default, nil); err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestCustomShellAllowRuleSuppressesPrompt(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "shell-allow.txt"), []byte(`^echo trusted`), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	manager, err := New(runtimeDir, t.TempDir(), Default, func(context.Context, Request) Decision {
		calls++
		return Deny
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.AuthorizeCommand(context.Background(), "echo", []string{"trusted"}, "test", 30) != AllowOnce || calls != 0 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestDefaultAllowedToolSuppressesPrompt(t *testing.T) {
	calls := 0
	manager := manager(t, Default, func(context.Context, Request) Decision {
		calls++
		return Deny
	})
	if manager.Authorize(context.Background(), Request{Tool: "webSearch", SimilarKey: "latest news"}) != AllowOnce || calls != 0 {
		t.Fatalf("calls = %d", calls)
	}
	if manager.Authorize(context.Background(), Request{Tool: "fetchURL", SimilarKey: "https://example.com"}) != AllowOnce || calls != 0 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestDefaultAllowedToolsFileIsEditable(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "default-allow-tools.txt"), []byte("webSearch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	manager, err := New(runtimeDir, t.TempDir(), Default, func(context.Context, Request) Decision {
		calls++
		return Deny
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.Authorize(context.Background(), Request{Tool: "webSearch"}) != AllowOnce {
		t.Fatal("webSearch was not allowed")
	}
	if manager.Authorize(context.Background(), Request{Tool: "fetchURL"}) != Deny || calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestDefaultAllowedCommandsFileIsEditable(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "default-allow-commands.txt"), []byte("# no default commands\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	calls := 0
	manager, err := New(runtimeDir, t.TempDir(), Default, func(context.Context, Request) Decision {
		calls++
		return Deny
	})
	if err != nil {
		t.Fatal(err)
	}
	if manager.AuthorizeCommand(context.Background(), "ls", []string{"."}, "inspect", 30) != Deny || calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestAuthorizeEmitsAuditEvents(t *testing.T) {
	manager := manager(t, Default, func(context.Context, Request) Decision { return Deny })
	var events []string
	manager.SetAudit(func(event string, _ any) { events = append(events, event) })
	manager.Authorize(context.Background(), Request{Tool: "downloadFile", SimilarKey: "x"})
	if len(events) != 2 || events[0] != "permission_requested" || events[1] != "permission_decision" {
		t.Fatalf("events = %#v", events)
	}
}
