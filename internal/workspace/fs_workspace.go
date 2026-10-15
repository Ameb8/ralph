package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	ralphErrors "ralph/internal/errors"
)

var specRegex = regexp.MustCompile(`docs/specs/[A-Za-z0-9_.\-/]+\.md`)

// LockData represents the serialized contents of .ralph-lock.
type LockData struct {
	PID       int       `json:"pid"`
	Issues    []int     `json:"issues"`
	StartTime time.Time `json:"start_time"`
}

// FSWorkspace implements Workspace on the local filesystem.
type FSWorkspace struct {
	repoRoot     string
	pollInterval time.Duration
}

// NewFSWorkspace creates a new FSWorkspace rooted at repoRoot.
func NewFSWorkspace(repoRoot string) *FSWorkspace {
	if repoRoot == "" {
		repoRoot = "."
	}
	return &FSWorkspace{
		repoRoot:     repoRoot,
		pollInterval: 5 * time.Second,
	}
}

// SetPollInterval overrides the polling interval (primarily for fast unit tests).
func (w *FSWorkspace) SetPollInterval(interval time.Duration) {
	w.pollInterval = interval
}

// lockFilePath returns the path to .ralph-lock.
func (w *FSWorkspace) lockFilePath() string {
	return filepath.Join(w.repoRoot, ".ralph-lock")
}

// handoffFilePath returns the path to .ralph-handoff.md.
func (w *FSWorkspace) handoffFilePath() string {
	return filepath.Join(w.repoRoot, ".ralph-handoff.md")
}

// signalFilePath returns the path to ralph.<signal>.
func (w *FSWorkspace) signalFilePath(sig SignalType) string {
	return filepath.Join(w.repoRoot, fmt.Sprintf("ralph.%s", sig))
}

// isProcessRunning checks if a PID is alive on the host.
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, sending signal 0 checks if the process exists and we have permission
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// AcquireLock writes .ralph-lock at the repo root.
// If a lockfile exists, it checks if the recorded process is still running.
// If running, it returns an error; if stale, it removes the lockfile and proceeds.
func (w *FSWorkspace) AcquireLock(issueNums []int) error {
	lockPath := w.lockFilePath()

	if data, err := os.ReadFile(lockPath); err == nil {
		var lock LockData
		if jsonErr := json.Unmarshal(data, &lock); jsonErr == nil && lock.PID > 0 {
			if isProcessRunning(lock.PID) {
				return ralphErrors.NewSystemError(fmt.Errorf("another Ralph process (PID %d) is currently running for issues %v", lock.PID, lock.Issues))
			}
			// Stale lockfile: clean it up
			_ = os.Remove(lockPath)
		} else {
			// Unparseable lockfile: clean it up
			_ = os.Remove(lockPath)
		}
	}

	lock := LockData{
		PID:       os.Getpid(),
		Issues:    issueNums,
		StartTime: time.Now().UTC(),
	}

	bytes, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return ralphErrors.NewSystemError(fmt.Errorf("failed to encode lock data: %w", err))
	}

	if err := os.WriteFile(lockPath, bytes, 0644); err != nil {
		return ralphErrors.NewSystemError(fmt.Errorf("failed to write lock file: %w", err))
	}

	return nil
}

// ReleaseLock removes .ralph-lock from the repo root.
func (w *FSWorkspace) ReleaseLock() error {
	lockPath := w.lockFilePath()
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		return ralphErrors.NewSystemError(fmt.Errorf("failed to remove lock file: %w", err))
	}
	return nil
}

// InitializeHandoff creates an empty .ralph-handoff.md file at the repository root.
func (w *FSWorkspace) InitializeHandoff() error {
	handoffPath := w.handoffFilePath()
	if err := os.WriteFile(handoffPath, []byte(""), 0644); err != nil {
		return ralphErrors.NewSystemError(fmt.Errorf("failed to initialize handoff file: %w", err))
	}
	return nil
}

// ReadHandoff reads the contents of .ralph-handoff.md.
func (w *FSWorkspace) ReadHandoff() (string, error) {
	handoffPath := w.handoffFilePath()
	data, err := os.ReadFile(handoffPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", ralphErrors.NewSystemError(fmt.Errorf("failed to read handoff file: %w", err))
	}
	return string(data), nil
}

// ClearSignals deletes ralph.done and ralph.escalate to avoid misreading stale signals.
func (w *FSWorkspace) ClearSignals() error {
	_ = os.Remove(w.signalFilePath(SignalDone))
	_ = os.Remove(w.signalFilePath(SignalEscalate))
	return nil
}

// ReadSignal reads the content of ralph.<signal>, retrying briefly if the file is momentarily empty during write.
func (w *FSWorkspace) ReadSignal(sig SignalType) (string, error) {
	path := w.signalFilePath(sig)
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			time.Sleep(10 * time.Millisecond)
			continue
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed != "" || attempt == 9 {
			return trimmed, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", nil
}

// WaitForSignal polls the workspace for ralph.done or ralph.escalate.
func (w *FSWorkspace) WaitForSignal(ctx context.Context) (SignalType, error) {
	interval := w.pollInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check before waiting
	if _, err := os.Stat(w.signalFilePath(SignalEscalate)); err == nil {
		return SignalEscalate, nil
	}
	if _, err := os.Stat(w.signalFilePath(SignalDone)); err == nil {
		return SignalDone, nil
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if _, err := os.Stat(w.signalFilePath(SignalEscalate)); err == nil {
				return SignalEscalate, nil
			}
			if _, err := os.Stat(w.signalFilePath(SignalDone)); err == nil {
				return SignalDone, nil
			}
		}
	}
}

// ExtractSpecPaths finds all docs/specs/*.md occurrences in an issue body.
func (w *FSWorkspace) ExtractSpecPaths(issueBody string) []string {
	matches := specRegex.FindAllString(issueBody, -1)
	seen := make(map[string]bool)
	var paths []string
	for _, m := range matches {
		clean := strings.Trim(m, "`'\"")
		if !seen[clean] {
			seen[clean] = true
			paths = append(paths, clean)
		}
	}
	return paths
}

// ExtractSpecs reads all referenced spec files from the repository root.
func (w *FSWorkspace) ExtractSpecs(repoRoot string, specPaths []string) (map[string]string, error) {
	if repoRoot == "" {
		repoRoot = w.repoRoot
	}
	specs := make(map[string]string)
	for _, p := range specPaths {
		fullPath := filepath.Join(repoRoot, p)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, ralphErrors.NewEscalationError(fmt.Sprintf("referenced spec file missing on disk: %s", p))
			}
			return nil, ralphErrors.NewSystemError(fmt.Errorf("failed to read spec file %s: %w", p, err))
		}
		specs[p] = string(data)
	}
	return specs, nil
}
