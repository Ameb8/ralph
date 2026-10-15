package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ralph/internal/config"
	ralphErrors "ralph/internal/errors"
)

func TestLoadDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `{
  "agent": {
    "type": "codex",
    "codex": {
      "command": "codex",
      "model": "o3",
      "reasoning_effort": "high",
      "sandbox": "workspace-write",
      "extra_args": ["--verbose"]
    },
    "env": {
      "FOO": "bar"
    }
  },
  "max_retries": 5
}`
	configPath := filepath.Join(tmpDir, "ralph.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Agent.Type != "codex" {
		t.Errorf("expected agent type codex, got %s", cfg.Agent.Type)
	}
	if cfg.Agent.Codex.Model != "o3" {
		t.Errorf("expected model o3, got %s", cfg.Agent.Codex.Model)
	}
	if cfg.Agent.Codex.ReasoningEffort != "high" {
		t.Errorf("expected reasoning effort high, got %s", cfg.Agent.Codex.ReasoningEffort)
	}
	if cfg.Agent.Codex.Sandbox != "workspace-write" {
		t.Errorf("expected sandbox workspace-write, got %s", cfg.Agent.Codex.Sandbox)
	}
	if len(cfg.Agent.Codex.ExtraArgs) != 1 || cfg.Agent.Codex.ExtraArgs[0] != "--verbose" {
		t.Errorf("expected extra_args [--verbose], got %v", cfg.Agent.Codex.ExtraArgs)
	}
	if cfg.Agent.Env["FOO"] != "bar" && cfg.Agent.Env["foo"] != "bar" {
		t.Errorf("expected env FOO=bar, got %v", cfg.Agent.Env)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected max_retries 5, got %d", cfg.MaxRetries)
	}
}

func TestLoadDefaultsWhenOmitted(t *testing.T) {
	tmpDir := t.TempDir()
	configContent := `{}`
	configPath := filepath.Join(tmpDir, "ralph.json")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Agent.Type != "codex" {
		t.Errorf("expected default agent type codex, got %s", cfg.Agent.Type)
	}
	if cfg.Agent.Codex.Command != "codex" {
		t.Errorf("expected default command codex, got %s", cfg.Agent.Codex.Command)
	}
	if cfg.Agent.Codex.Sandbox != "workspace-write" {
		t.Errorf("expected default sandbox workspace-write, got %s", cfg.Agent.Codex.Sandbox)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected default max_retries 3, got %d", cfg.MaxRetries)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load("/non/existent/path/ralph.json")
	if err == nil {
		t.Fatalf("expected error for non-existent config, got nil")
	}
	if !errors.Is(err, ralphErrors.ErrSystem) {
		t.Errorf("expected ErrSystem, got %v", err)
	}
}
