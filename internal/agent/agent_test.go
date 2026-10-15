package agent_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ralph/internal/agent"
)

func TestFactory(t *testing.T) {
	// Test default codex
	r, err := agent.NewRunner(&agent.Config{})
	if err != nil {
		t.Fatalf("unexpected error creating default runner: %v", err)
	}
	if _, ok := r.(*agent.CodexRunner); !ok {
		t.Fatalf("expected *agent.CodexRunner, got %T", r)
	}

	// Test explicit codex
	r, err = agent.NewRunner(&agent.Config{Type: "codex"})
	if err != nil {
		t.Fatalf("unexpected error creating explicit codex runner: %v", err)
	}
	if _, ok := r.(*agent.CodexRunner); !ok {
		t.Fatalf("expected *agent.CodexRunner, got %T", r)
	}

	// Test unsupported agent type
	_, err = agent.NewRunner(&agent.Config{Type: "claude"})
	if err == nil {
		t.Fatalf("expected error for unsupported agent type claude")
	}
	expectedMsg := `unsupported agent type "claude": V0.1.0 only supports 'codex'`
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("expected error message to contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestCodexRunnerBuildArgs(t *testing.T) {
	runner := agent.NewCodexRunner(&agent.CodexConfig{
		Command:         "codex",
		Model:           "o3",
		ReasoningEffort: "high",
		Sandbox:         "workspace-write",
		ExtraArgs:       []string{"--dummy-flag", "val"},
	}, nil)
	runner.SetRepoRoot("/tmp/repo")

	args := runner.BuildArgs("test prompt")
	expected := []string{
		"exec",
		"-C", "/tmp/repo",
		"-s", "workspace-write",
		"-m", "o3",
		"-c", `model_reasoning_effort="high"`,
		"--dummy-flag", "val",
		"test prompt",
	}

	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, arg := range args {
		if arg != expected[i] {
			t.Errorf("arg[%d] expected %q, got %q", i, expected[i], arg)
		}
	}
}

func TestCodexRunnerRunSessionLogsAndCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	mockCodex := filepath.Join(tmpDir, "mock-codex.sh")
	if err := os.WriteFile(mockCodex, []byte("#!/bin/sh\necho 'mock agent output'\nexit 0\n"), 0755); err != nil {
		t.Fatalf("failed to create mock codex script: %v", err)
	}

	runner := agent.NewCodexRunner(&agent.CodexConfig{
		Command: mockCodex,
		Sandbox: "workspace-write",
	}, map[string]string{"CUSTOM_VAR": "custom_val"})
	runner.SetRepoRoot(tmpDir)

	ctx := context.Background()
	err := runner.RunSession(ctx, "test-run", "ignored prompt")
	if err != nil {
		t.Fatalf("unexpected error running mock session: %v", err)
	}

	// Verify log file was created
	logFile := filepath.Join(tmpDir, ".ralph", "runs", "test-run", "agent.log")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read agent.log: %v", err)
	}
	if !strings.Contains(string(content), "mock agent output") {
		t.Errorf("expected log to contain 'mock agent output', got %q", string(content))
	}

	// Test context cancellation
	mockSlowCodex := filepath.Join(tmpDir, "mock-slow-codex.sh")
	if err := os.WriteFile(mockSlowCodex, []byte("#!/bin/sh\nsleep 5\n"), 0755); err != nil {
		t.Fatalf("failed to create mock slow codex script: %v", err)
	}

	cancelCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	slowRunner := agent.NewCodexRunner(&agent.CodexConfig{
		Command: mockSlowCodex,
	}, nil)
	slowRunner.SetRepoRoot(tmpDir)

	err = slowRunner.RunSession(cancelCtx, "test-cancel", "ignored prompt")
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}
