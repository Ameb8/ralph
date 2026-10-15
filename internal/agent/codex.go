package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CodexRunner executes OpenAI Codex CLI (`codex exec`).
type CodexRunner struct {
	cfg      CodexConfig
	env      map[string]string
	repoRoot string
}

// NewCodexRunner creates a new CodexRunner with the provided configuration and environment.
func NewCodexRunner(cfg *CodexConfig, env map[string]string) *CodexRunner {
	c := CodexConfig{
		Command: "codex",
		Sandbox: "workspace-write",
	}
	if cfg != nil {
		if cfg.Command != "" {
			c.Command = cfg.Command
		}
		c.Model = cfg.Model
		c.ReasoningEffort = cfg.ReasoningEffort
		if cfg.Sandbox != "" {
			c.Sandbox = cfg.Sandbox
		}
		c.ExtraArgs = cfg.ExtraArgs
	}

	return &CodexRunner{
		cfg:      c,
		env:      env,
		repoRoot: ".",
	}
}

// SetRepoRoot sets the root directory for repository execution.
func (r *CodexRunner) SetRepoRoot(root string) {
	r.repoRoot = root
}

// BuildArgs constructs the command-line argument slice for codex exec.
func (r *CodexRunner) BuildArgs(prompt string) []string {
	args := []string{"exec"}

	repoDir := r.repoRoot
	if repoDir == "" {
		repoDir = "."
	}
	absRepoDir, err := filepath.Abs(repoDir)
	if err == nil {
		repoDir = absRepoDir
	}

	args = append(args, "-C", repoDir)

	sandbox := r.cfg.Sandbox
	if sandbox == "" {
		sandbox = "workspace-write"
	}
	args = append(args, "-s", sandbox)

	if r.cfg.Model != "" {
		args = append(args, "-m", r.cfg.Model)
	}

	if r.cfg.ReasoningEffort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", r.cfg.ReasoningEffort))
	}

	if len(r.cfg.ExtraArgs) > 0 {
		args = append(args, r.cfg.ExtraArgs...)
	}

	args = append(args, prompt)
	return args
}

// RunSession executes a single agent session non-interactively with the provided prompt.
// Output (stdout & stderr) is persisted to .ralph/runs/<runID>/agent.log.
func (r *CodexRunner) RunSession(ctx context.Context, runID string, prompt string) error {
	runDir := filepath.Join(r.repoRoot, ".ralph", "runs", runID)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return fmt.Errorf("failed to create run directory %s: %w", runDir, err)
	}

	logPath := filepath.Join(runDir, "agent.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}
	defer logFile.Close()

	cmdName := r.cfg.Command
	if cmdName == "" {
		cmdName = "codex"
	}

	args := r.BuildArgs(prompt)
	cmd := exec.CommandContext(ctx, cmdName, args...)
	cmd.Dir = r.repoRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Merge environment
	cmd.Env = os.Environ()
	for k, v := range r.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent process exited with error: %w", err)
	}

	return nil
}
