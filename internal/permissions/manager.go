package permissions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Mode string

const (
	Ask     Mode = "ask"
	Default Mode = "default"
	Allow   Mode = "allow"
)

type Decision string

const (
	AllowOnce    Decision = "allow_once"
	AllowSimilar Decision = "allow_similar"
	Deny         Decision = "deny"
)

type Request struct {
	Tool       string
	Reason     string
	Target     string
	SimilarKey string
	Details    string
	Elevated   bool
}

type PromptFunc func(context.Context, Request) Decision

type Manager struct {
	mu        sync.RWMutex
	mode      Mode
	prompt    PromptFunc
	rules     map[string]bool
	allow     []*regexp.Regexp
	deny      []*regexp.Regexp
	workspace string
	audit     func(string, any)
}

func (m *Manager) SetAudit(audit func(string, any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audit = audit
}

func New(runtimeDir, workspace string, mode Mode, prompt PromptFunc) (*Manager, error) {
	if mode != Ask && mode != Default && mode != Allow {
		return nil, fmt.Errorf("invalid permission mode: %s", mode)
	}
	if err := ensureRuleFiles(runtimeDir); err != nil {
		return nil, err
	}
	allow, err := loadRegexFile(filepath.Join(runtimeDir, "shell-allow.txt"))
	if err != nil {
		return nil, err
	}
	deny, err := loadRegexFile(filepath.Join(runtimeDir, "shell-deny.txt"))
	if err != nil {
		return nil, err
	}
	return &Manager{mode: mode, prompt: prompt, rules: map[string]bool{}, allow: allow, deny: deny, workspace: workspace}, nil
}

func (m *Manager) Mode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

func (m *Manager) SetMode(mode Mode) error {
	if mode != Ask && mode != Default && mode != Allow {
		return fmt.Errorf("invalid permission mode: %s", mode)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
	return nil
}

func (m *Manager) Authorize(ctx context.Context, request Request) Decision {
	m.mu.RLock()
	mode := m.mode
	remembered := m.rules[request.Tool+"\x00"+request.SimilarKey]
	prompt := m.prompt
	audit := m.audit
	m.mu.RUnlock()
	if !request.Elevated {
		if mode == Allow || (mode == Default && remembered) {
			if audit != nil {
				audit("permission_allowed", map[string]any{"request": request, "source": "mode_or_rule"})
			}
			return AllowOnce
		}
	}
	if prompt == nil {
		if audit != nil {
			audit("permission_denied", map[string]any{"request": request, "source": "no_prompt"})
		}
		return Deny
	}
	if audit != nil {
		audit("permission_requested", request)
	}
	decision := prompt(ctx, request)
	if decision == AllowSimilar {
		m.mu.Lock()
		m.rules[request.Tool+"\x00"+request.SimilarKey] = true
		m.mu.Unlock()
	}
	if audit != nil {
		audit("permission_decision", map[string]any{"request": request, "decision": decision})
	}
	return decision
}

func (m *Manager) AuthorizeCommand(ctx context.Context, command string, args []string, reason string, timeoutSeconds int) Decision {
	normalized := NormalizeCommand(command, args)
	if matches(m.deny, normalized) {
		m.auditEvent("permission_denied", map[string]any{"tool": "runCommand", "target": normalized, "source": "deny_rule"})
		return Deny
	}
	elevated := timeoutSeconds > 30
	if elevated || m.Mode() == Ask {
		return m.Authorize(ctx, Request{
			Tool: "runCommand", Reason: reason, Target: normalized, SimilarKey: shellSimilarKey(command, args),
			Details: fmt.Sprintf("timeout: %ds", timeoutSeconds), Elevated: elevated,
		})
	}
	if m.Mode() == Allow {
		m.auditEvent("permission_allowed", map[string]any{"tool": "runCommand", "target": normalized, "source": "allow_mode"})
		return AllowOnce
	}
	if matches(m.allow, normalized) || m.safeInspection(command, args) {
		m.auditEvent("permission_allowed", map[string]any{"tool": "runCommand", "target": normalized, "source": "shell_rule"})
		return AllowOnce
	}
	return m.Authorize(ctx, Request{
		Tool: "runCommand", Reason: reason, Target: normalized, SimilarKey: shellSimilarKey(command, args),
		Details: fmt.Sprintf("timeout: %ds", timeoutSeconds),
	})
}

func shellSimilarKey(command string, args []string) string {
	key := filepath.Base(command)
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		key += " " + args[0]
	}
	return key
}

func (m *Manager) auditEvent(event string, data any) {
	m.mu.RLock()
	audit := m.audit
	m.mu.RUnlock()
	if audit != nil {
		audit(event, data)
	}
}

func NormalizeCommand(command string, args []string) string {
	return strings.TrimSpace(command + " " + strings.Join(args, " "))
}

func (m *Manager) safeInspection(command string, args []string) bool {
	base := filepath.Base(command)
	if base == "pwd" && len(args) == 0 {
		return true
	}
	switch base {
	case "ls", "cat", "head", "tail", "wc", "rg", "find":
	default:
		return false
	}
	for _, arg := range args {
		if strings.Contains(arg, "..") || filepath.IsAbs(arg) {
			return false
		}
		if base == "find" && (arg == "-delete" || arg == "-exec" || arg == "-execdir" || arg == "-ok" || arg == "-okdir") {
			return false
		}
		if base == "rg" && (arg == "--pre" || strings.HasPrefix(arg, "--pre=")) {
			return false
		}
	}
	return true
}

func ensureRuleFiles(runtimeDir string) error {
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		return err
	}
	allow := filepath.Join(runtimeDir, "shell-allow.txt")
	if _, err := os.Stat(allow); os.IsNotExist(err) {
		if err := os.WriteFile(allow, []byte("# One trusted shell-command regex per line.\n"), 0o644); err != nil {
			return err
		}
	}
	deny := filepath.Join(runtimeDir, "shell-deny.txt")
	if _, err := os.Stat(deny); os.IsNotExist(err) {
		content := `# Hard-denied shell-command regexes. Deny rules override all modes.
^(sudo|su)( |$)
^(shutdown|reboot|halt|poweroff)( |$)
^(mkfs|fdisk|diskutil erase|dd)( |$)
^rm .*?((-rf|-fr|--recursive).*?(/|~|\$HOME)|(/|~|\$HOME).*?(-rf|-fr|--recursive))( |$)
`
		if err := os.WriteFile(deny, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func loadRegexFile(name string) ([]*regexp.Regexp, error) {
	content, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var result []*regexp.Regexp
	for lineNumber, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		compiled, err := regexp.Compile(line)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: invalid regex: %w", name, lineNumber+1, err)
		}
		result = append(result, compiled)
	}
	return result, nil
}

func matches(patterns []*regexp.Regexp, value string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}
