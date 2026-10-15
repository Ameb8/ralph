package workspace_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	ralphErrors "ralph/internal/errors"
	"ralph/internal/workspace"
)

func TestLockLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	ws := workspace.NewFSWorkspace(tmpDir)

	// Acquire lock initially
	if err := ws.AcquireLock([]int{42, 45}); err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Try acquiring again with current live process PID (should fail)
	err := ws.AcquireLock([]int{42, 45})
	if err == nil {
		t.Fatalf("expected error when lock is already held by live process, got nil")
	}

	// Release lock
	if err := ws.ReleaseLock(); err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Stale lock test: write lock with non-existent PID (e.g. 9999999)
	lockFile := filepath.Join(tmpDir, ".ralph-lock")
	staleContent := `{"pid": 9999999, "issues": [1], "start_time": "2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(lockFile, []byte(staleContent), 0644); err != nil {
		t.Fatalf("failed to write stale lock: %v", err)
	}

	// Should detect stale lock and acquire cleanly
	if err := ws.AcquireLock([]int{42}); err != nil {
		t.Fatalf("failed to acquire lock over stale lock: %v", err)
	}
	_ = ws.ReleaseLock()
}

func TestHandoffLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	ws := workspace.NewFSWorkspace(tmpDir)

	// Read before init
	handoff, err := ws.ReadHandoff()
	if err != nil {
		t.Fatalf("unexpected error reading missing handoff: %v", err)
	}
	if handoff != "" {
		t.Errorf("expected empty string for missing handoff, got %q", handoff)
	}

	// Initialize
	if err := ws.InitializeHandoff(); err != nil {
		t.Fatalf("failed to init handoff: %v", err)
	}

	// Append note
	handoffFile := filepath.Join(tmpDir, ".ralph-handoff.md")
	if err := os.WriteFile(handoffFile, []byte("### Work Done\n- Added auth middleware"), 0644); err != nil {
		t.Fatalf("failed to write handoff: %v", err)
	}

	content, err := ws.ReadHandoff()
	if err != nil {
		t.Fatalf("failed to read handoff: %v", err)
	}
	if content != "### Work Done\n- Added auth middleware" {
		t.Errorf("unexpected handoff content: %q", content)
	}
}

func TestSignalsAndPolling(t *testing.T) {
	tmpDir := t.TempDir()
	ws := workspace.NewFSWorkspace(tmpDir)
	ws.SetPollInterval(10 * time.Millisecond)

	// Clear signals initially
	_ = ws.ClearSignals()

	// Test WaitForSignal Done
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(tmpDir, "ralph.done"), []byte("success"), 0644)
	}()

	sig, err := ws.WaitForSignal(ctx)
	if err != nil {
		t.Fatalf("unexpected error waiting for signal: %v", err)
	}
	if sig != workspace.SignalDone {
		t.Errorf("expected SignalDone, got %v", sig)
	}

	msg, err := ws.ReadSignal(workspace.SignalDone)
	if err != nil {
		t.Fatalf("failed to read signal: %v", err)
	}
	if msg != "success" {
		t.Errorf("expected 'success', got %q", msg)
	}

	// Clear and test escalate
	_ = ws.ClearSignals()

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(tmpDir, "ralph.escalate"), []byte("Blocked on missing DB dependency"), 0644)
	}()

	sig, err = ws.WaitForSignal(ctx)
	if err != nil {
		t.Fatalf("unexpected error waiting for signal: %v", err)
	}
	if sig != workspace.SignalEscalate {
		t.Errorf("expected SignalEscalate, got %v", sig)
	}

	escMsg, err := ws.ReadSignal(workspace.SignalEscalate)
	if err != nil {
		t.Fatalf("failed to read escalate signal: %v", err)
	}
	if escMsg != "Blocked on missing DB dependency" {
		t.Errorf("unexpected escalate message: %q", escMsg)
	}

	// Test context cancellation timeout
	_ = ws.ClearSignals()
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer timeoutCancel()

	_, err = ws.WaitForSignal(timeoutCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestExtractSpecPathsAndSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	ws := workspace.NewFSWorkspace(tmpDir)

	issueBody := `
## Summary
Implement feature

## Specs
- docs/specs/feature-a.md
- ` + "`docs/specs/sub/feature-b.md`" + `
`
	paths := ws.ExtractSpecPaths(issueBody)
	if len(paths) != 2 {
		t.Fatalf("expected 2 spec paths, got %d: %v", len(paths), paths)
	}

	// Create the files on disk
	specDirA := filepath.Join(tmpDir, "docs", "specs")
	specDirB := filepath.Join(tmpDir, "docs", "specs", "sub")
	if err := os.MkdirAll(specDirB, 0755); err != nil {
		t.Fatalf("failed to create spec dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDirA, "feature-a.md"), []byte("# Feature A Spec"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDirB, "feature-b.md"), []byte("# Feature B Spec"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	specs, err := ws.ExtractSpecs(tmpDir, paths)
	if err != nil {
		t.Fatalf("failed to extract specs: %v", err)
	}
	if specs["docs/specs/feature-a.md"] != "# Feature A Spec" {
		t.Errorf("unexpected spec content: %v", specs["docs/specs/feature-a.md"])
	}
	if specs["docs/specs/sub/feature-b.md"] != "# Feature B Spec" {
		t.Errorf("unexpected spec content: %v", specs["docs/specs/sub/feature-b.md"])
	}

	// Missing spec should return ErrEscalation
	_, err = ws.ExtractSpecs(tmpDir, []string{"docs/specs/missing.md"})
	if err == nil {
		t.Fatalf("expected error for missing spec file, got nil")
	}
	if !errors.Is(err, ralphErrors.ErrEscalation) {
		t.Errorf("expected ErrEscalation for missing spec, got %v", err)
	}
}
