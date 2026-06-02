package definitions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

const browserOutputLimit = 64 << 10

type BrowserRunner interface {
	Run(context.Context, map[string]any) (map[string]any, error)
}

type CloakBrowserRunner struct {
	Python string
	Bridge string
}

func NewCloakBrowserRunner() CloakBrowserRunner {
	python := os.Getenv("OPENHOME_CLOAKBROWSER_PYTHON")
	if python == "" {
		python = filepath.Join(".venv", "bin", "python")
		if _, err := os.Stat(python); err != nil {
			python = "python3"
		}
	}
	bridge := os.Getenv("OPENHOME_CLOAKBROWSER_BRIDGE")
	if bridge == "" {
		bridge = filepath.Join("scripts", "cloakbrowser_bridge.py")
	}
	return CloakBrowserRunner{Python: python, Bridge: bridge}
}

func BrowserInteract(runner BrowserRunner) tools.Definition {
	return tools.Definition{
		Name: "browserInteract", Advanced: true,
		Description: "Open a website in CloakBrowser, optionally click or type using CSS selectors, capture a workspace screenshot, and return compact visible page text and links. Browser interaction always requires permission.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"url", "reason"},
			"properties": map[string]any{
				"url":                map[string]any{"type": "string"},
				"reason":             map[string]any{"type": "string"},
				"screenshotPath":     map[string]any{"type": "string", "description": "Optional workspace-relative PNG output path."},
				"screenshotSelector": map[string]any{"type": "string", "description": "Optional CSS selector to screenshot instead of the full page or viewport."},
				"screenshotHideSelectors": map[string]any{
					"type": "array", "maxItems": 10, "items": map[string]any{"type": "string"},
					"description": "Optional CSS selectors to hide immediately before screenshot without clicking or changing site state.",
				},
				"fullPage": map[string]any{"type": "boolean", "description": "Capture the full page when screenshotPath is set. Defaults to false."},
				"actions": map[string]any{"type": "array", "maxItems": 12, "items": map[string]any{
					"type": "object", "additionalProperties": false, "required": []string{"action"},
					"properties": map[string]any{
						"action":   map[string]any{"type": "string", "enum": []string{"click", "fill", "type", "press", "wait", "goto"}},
						"selector": map[string]any{"type": "string"},
						"value":    map[string]any{"type": "string"},
						"url":      map[string]any{"type": "string"},
						"ms":       map[string]any{"type": "integer", "minimum": 0, "maximum": 10000},
					},
				}},
			},
		},
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			delete(args, "screenshotOutputPath")
			rawURL, err := stringArg(args, "url")
			if err != nil {
				return nil, err
			}
			parsed, err := parseHTTPURL(rawURL)
			if err != nil {
				return nil, err
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			if err := validateBrowserActions(args["actions"]); err != nil {
				return nil, err
			}
			screenshotPath, err := optionalStringArg(args, "screenshotPath", "")
			if err != nil {
				return nil, err
			}
			screenshotSelector, err := optionalStringArg(args, "screenshotSelector", "")
			if err != nil {
				return nil, err
			}
			if _, err := boolArg(args, "fullPage", false); err != nil {
				return nil, err
			}
			if screenshotSelector != "" && screenshotPath == "" {
				return nil, fmt.Errorf("screenshotSelector requires screenshotPath")
			}
			hideSelectors, err := stringSliceArg(args, "screenshotHideSelectors")
			if err != nil || len(hideSelectors) > 10 {
				return nil, fmt.Errorf("screenshotHideSelectors must contain at most 10 strings")
			}
			if len(hideSelectors) > 0 && screenshotPath == "" {
				return nil, fmt.Errorf("screenshotHideSelectors requires screenshotPath")
			}
			if screenshotPath != "" {
				if !strings.EqualFold(filepath.Ext(screenshotPath), ".png") {
					return nil, fmt.Errorf("screenshotPath must end in .png")
				}
				resolved, err := toolContext.Workspace.ResolveWrite(screenshotPath)
				if err != nil {
					return nil, err
				}
				args["screenshotOutputPath"] = resolved
			}
			if toolContext.Permissions == nil || toolContext.Permissions.Authorize(ctx, permissions.Request{
				Tool: "browserInteract", Reason: reason, Target: parsed.String(), SimilarKey: parsed.Host,
				Details: browserDetails(screenshotPath), Elevated: true,
			}) == permissions.Deny {
				return denied("browserInteract"), nil
			}
			result, err := runner.Run(ctx, args)
			if err == nil && screenshotPath != "" {
				result["screenshotPath"] = screenshotPath
			}
			return result, err
		},
	}
}

func browserDetails(screenshotPath string) string {
	details := "CloakBrowser may open pages, click elements, or type text."
	if screenshotPath != "" {
		details += " Screenshot output: " + screenshotPath
	}
	return details
}

func validateBrowserActions(value any) error {
	if value == nil {
		return nil
	}
	actions, ok := value.([]any)
	if !ok || len(actions) > 12 {
		return fmt.Errorf("actions must be an array with at most 12 items")
	}
	for _, value := range actions {
		action, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("each browser action must be an object")
		}
		kind, err := stringArg(action, "action")
		if err != nil {
			return err
		}
		switch kind {
		case "click", "fill", "type", "press":
			if _, err := stringArg(action, "selector"); err != nil {
				return fmt.Errorf("%s action requires selector", kind)
			}
		case "wait":
			ms, err := intArg(action, "ms", 500)
			if err != nil || ms < 0 || ms > 10000 {
				return fmt.Errorf("wait action ms must be between 0 and 10000")
			}
		case "goto":
			rawURL, err := stringArg(action, "url")
			if err != nil {
				return fmt.Errorf("goto action requires url")
			}
			if _, err := parseHTTPURL(rawURL); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported browser action: %s", kind)
		}
	}
	return nil
}

func (runner CloakBrowserRunner) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	content, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, runner.Python, runner.Bridge)
	command.Stdin = bytes.NewReader(content)
	stdout := &cappedBuffer{limit: browserOutputLimit}
	stderr := &cappedBuffer{limit: browserOutputLimit}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		message := stderr.buffer.String()
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("CloakBrowser bridge failed: %s", message)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.buffer.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("decode CloakBrowser bridge output: %w", err)
	}
	return result, nil
}
