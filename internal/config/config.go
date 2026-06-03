package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	OpenAIBaseURL   string
	OpenAIModel     string
	OpenAIAPIKey    string
	SkillsDir       string
	WorkspaceDir    string
	RunsDir         string
	RuntimeDir      string
	PermissionMode  string
	MaxToolRounds   int
	MaxContextBytes int
	// Deprecated: retained for launch-script compatibility after per-step verification removal.
	MaxStepAttempts int
}

func Load() (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}
	return LoadFrom(cwd)
}

func LoadFrom(cwd string) (Config, error) {
	rounds, err := nonNegativeInteger("OPENHOME_MAX_TOOL_ROUNDS", 0)
	if err != nil {
		return Config{}, err
	}
	attempts, err := positiveInteger("OPENHOME_MAX_STEP_ATTEMPTS", 3)
	if err != nil {
		return Config{}, err
	}
	contextBytes, err := positiveInteger("OPENHOME_MAX_CONTEXT_BYTES", 96<<10)
	if err != nil {
		return Config{}, err
	}
	return Config{
		OpenAIBaseURL:   env("OPENAI_BASE_URL", "http://10.10.30.80:8000/v1"),
		OpenAIModel:     env("OPENAI_MODEL", "gemma-4-e2b-it-bf16"),
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		SkillsDir:       resolve(cwd, env("OPENHOME_SKILLS_DIR", "skills")),
		WorkspaceDir:    resolve(cwd, env("OPENHOME_WORKSPACE", "workspace")),
		RunsDir:         resolve(cwd, ".openhome/runs"),
		RuntimeDir:      resolve(cwd, ".openhome"),
		PermissionMode:  env("OPENHOME_PERMISSION_MODE", "default"),
		MaxToolRounds:   rounds,
		MaxContextBytes: contextBytes,
		MaxStepAttempts: attempts,
	}, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func resolve(cwd, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(cwd, name)
}

func positiveInteger(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func nonNegativeInteger(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return parsed, nil
}
