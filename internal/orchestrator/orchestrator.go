package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ralph/internal/agent"
	ralphErrors "ralph/internal/errors"
	"ralph/internal/git"
	"ralph/internal/github"
	"ralph/internal/prompt"
	"ralph/internal/workspace"
)

type Config struct {
	MaxRetries int
}

type Orchestrator struct {
	github    github.GitHubClient
	git       git.GitClient
	agent     agent.AgentRunner
	workspace workspace.Workspace
	prompts   prompt.Assembler
	config    *Config
	repoRoot  string
}

func New(
	githubClient github.GitHubClient,
	gitClient git.GitClient,
	agentRunner agent.AgentRunner,
	ws workspace.Workspace,
	assembler prompt.Assembler,
	cfg *Config,
) *Orchestrator {
	if cfg == nil {
		cfg = &Config{MaxRetries: 3}
	}
	return &Orchestrator{
		github:    githubClient,
		git:       gitClient,
		agent:     agentRunner,
		workspace: ws,
		prompts:   assembler,
		config:    cfg,
		repoRoot:  ".",
	}
}

// SetRepoRoot sets the root directory for repository interactions.
func (o *Orchestrator) SetRepoRoot(root string) {
	o.repoRoot = root
}

// Run executes the unified PR vertical slice for the given issue numbers.
func (o *Orchestrator) Run(ctx context.Context, issueNumbers []int) error {
	if len(issueNumbers) == 0 {
		return ralphErrors.NewSystemError(fmt.Errorf("at least one issue number is required"))
	}

	// Acquire workspace lock
	if err := o.workspace.AcquireLock(issueNumbers); err != nil {
		return err
	}
	defer func() {
		_ = o.workspace.ReleaseLock()
	}()

	// 1. Create and checkout unified branch
	var issueStrs []string
	for _, n := range issueNumbers {
		issueStrs = append(issueStrs, strconv.Itoa(n))
	}
	branchName := fmt.Sprintf("ralph/unified-%s", strings.Join(issueStrs, "-"))
	if err := o.git.CreateAndCheckoutBranch(ctx, branchName); err != nil {
		return ralphErrors.NewSystemError(fmt.Errorf("failed to checkout branch %s: %w", branchName, err))
	}

	// 2. Initialize .ralph-handoff.md
	if err := o.workspace.InitializeHandoff(); err != nil {
		return ralphErrors.NewSystemError(err)
	}

	// Phase A: Sequential Issue Implementation
	var issues []*github.Issue
	for _, issueNum := range issueNumbers {
		fmt.Printf("Working on Issue #%d...\n", issueNum)

		issue, err := o.github.GetIssue(ctx, issueNum)
		if err != nil {
			return ralphErrors.NewSystemError(fmt.Errorf("failed to fetch issue #%d: %w", issueNum, err))
		}
		issues = append(issues, issue)

		specPaths := o.workspace.ExtractSpecPaths(issue.Body)
		specs, err := o.workspace.ExtractSpecs(o.repoRoot, specPaths)
		if err != nil {
			return err
		}

		handoff, err := o.workspace.ReadHandoff()
		if err != nil {
			return ralphErrors.NewSystemError(err)
		}

		promptText, err := o.prompts.BuildIssuePrompt(issue, specs, handoff)
		if err != nil {
			return ralphErrors.NewSystemError(err)
		}

		runID := fmt.Sprintf("issue-%d", issueNum)
		if err := o.executeSession(ctx, runID, promptText); err != nil {
			return err
		}
	}

	// Phase B: PR Drafting
	fmt.Println("Drafting PR...")
	handoff, err := o.workspace.ReadHandoff()
	if err != nil {
		return ralphErrors.NewSystemError(err)
	}

	draftPrompt, err := o.prompts.BuildDraftPRPrompt(issues, handoff)
	if err != nil {
		return ralphErrors.NewSystemError(err)
	}

	if err := o.executeSession(ctx, "draft-pr", draftPrompt); err != nil {
		return err
	}

	draftPRPath := filepath.Join(o.repoRoot, ".ralph", "draft-pr.md")
	data, err := os.ReadFile(draftPRPath)
	if err != nil {
		return ralphErrors.NewSystemError(fmt.Errorf("expected draft PR file at %s: %w", draftPRPath, err))
	}

	title := extractPRTitle(string(data), issueNumbers)
	prURL, err := o.github.CreatePR(ctx, title, draftPRPath, "")
	if err != nil {
		return ralphErrors.NewSystemError(fmt.Errorf("failed to create PR: %w", err))
	}
	if prURL != "" {
		fmt.Printf("Pull Request created: %s\n", prURL)
	}

	// Phase C: CI Polling & Retry Loop
	maxRetries := o.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		fmt.Println("Polling CI checks...")
		status, err := o.github.GetCIStatus(ctx, branchName)
		if err != nil {
			return ralphErrors.NewSystemError(fmt.Errorf("failed to get CI status: %w", err))
		}

		if status == github.CIStatusSuccess {
			fmt.Println("CI checks passed successfully.")
			return nil
		}

		// CI failure detected
		if attempt >= maxRetries {
			logs, _ := o.github.GetFailedLogs(ctx, branchName)
			return ralphErrors.NewCIFailureError(maxRetries, logs)
		}

		fmt.Printf("CI checks failed. Starting CI fix session (attempt %d/%d)...\n", attempt+1, maxRetries)

		logs, err := o.github.GetFailedLogs(ctx, branchName)
		if err != nil {
			logs = fmt.Sprintf("Failed to retrieve failure logs: %v", err)
		}

		handoff, _ := o.workspace.ReadHandoff()
		ciPrompt, err := o.prompts.BuildCIFixPrompt(logs, handoff)
		if err != nil {
			return ralphErrors.NewSystemError(err)
		}

		runID := fmt.Sprintf("ci-fix-%d", attempt+1)
		if err := o.executeSession(ctx, runID, ciPrompt); err != nil {
			return err
		}
	}

	return nil
}

func (o *Orchestrator) executeSession(ctx context.Context, runID string, promptText string) error {
	_ = o.workspace.ClearSignals()

	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()

	agentErrCh := make(chan error, 1)
	go func() {
		agentErrCh <- o.agent.RunSession(sessionCtx, runID, promptText)
	}()

	signalCh := make(chan struct {
		sig workspace.SignalType
		err error
	}, 1)
	go func() {
		sig, err := o.workspace.WaitForSignal(sessionCtx)
		signalCh <- struct {
			sig workspace.SignalType
			err error
		}{sig, err}
	}()

	select {
	case <-ctx.Done():
		cancelSession()
		return ctx.Err()

	case sigRes := <-signalCh:
		if sigRes.err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
		if sigRes.sig == workspace.SignalEscalate {
			cancelSession()
			reason, _ := o.workspace.ReadSignal(workspace.SignalEscalate)
			if reason == "" {
				reason = "agent signaled escalation without description"
			}
			return ralphErrors.NewEscalationError(reason)
		}
		if sigRes.sig == workspace.SignalDone {
			cancelSession()
			return nil
		}

	case agentErr := <-agentErrCh:
		// Agent process terminated. Check if escalate or done signal was written.
		if reason, err := o.workspace.ReadSignal(workspace.SignalEscalate); err == nil && reason != "" {
			return ralphErrors.NewEscalationError(reason)
		}
		if _, err := o.workspace.ReadSignal(workspace.SignalDone); err == nil {
			return nil
		}
		if agentErr != nil {
			return ralphErrors.NewEscalationError(fmt.Sprintf("agent exited with error: %v", agentErr))
		}
		return nil
	}

	return nil
}

func extractPRTitle(content string, issueNumbers []int) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			trimmed = strings.TrimPrefix(trimmed, "#")
			trimmed = strings.TrimSpace(trimmed)
			if trimmed != "" {
				return trimmed
			}
		}
	}

	var strs []string
	for _, n := range issueNumbers {
		strs = append(strs, fmt.Sprintf("#%d", n))
	}
	return fmt.Sprintf("Unified PR: %s", strings.Join(strs, ", "))
}
