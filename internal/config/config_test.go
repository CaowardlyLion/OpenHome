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
	t.Setenv("OPENHOME_MAX_CONTEXT_BYTES", "0")
	if _, err := LoadFrom(t.TempDir()); err == nil {
		t.Fatal("expected invalid context budget error")
	}
}

func TestLoadFromAllowsUnlimitedToolRounds(t *testing.T) {
	t.Setenv("OPENHOME_MAX_TOOL_ROUNDS", "0")
	cfg, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxToolRounds != 0 {
		t.Fatalf("MaxToolRounds = %d", cfg.MaxToolRounds)
	}
}

func TestLoadFromReadsContextBudget(t *testing.T) {
	t.Setenv("OPENHOME_MAX_CONTEXT_BYTES", "4096")
	cfg, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxContextBytes != 4096 {
		t.Fatalf("MaxContextBytes = %d", cfg.MaxContextBytes)
	}
}

func TestLoadFromUsesUnlimitedDefaultToolRounds(t *testing.T) {
	cfg, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxToolRounds != 0 {
		t.Fatalf("MaxToolRounds = %d", cfg.MaxToolRounds)
	}
}
