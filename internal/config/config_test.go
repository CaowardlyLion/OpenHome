package config

import "testing"

func TestLoadFromPreservesDeprecatedStepAttemptsSetting(t *testing.T) {
	t.Setenv("OPENHOME_MAX_STEP_ATTEMPTS", "5")
	cfg, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxStepAttempts != 5 {
		t.Fatalf("MaxStepAttempts = %d", cfg.MaxStepAttempts)
	}
}

func TestLoadFromRejectsInvalidPositiveInteger(t *testing.T) {
	t.Setenv("OPENHOME_MAX_TOOL_ROUNDS", "0")
	if _, err := LoadFrom(t.TempDir()); err == nil {
		t.Fatal("expected invalid rounds error")
	}
}
