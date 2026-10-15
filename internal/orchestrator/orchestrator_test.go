package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	ralphErrors "ralph/internal/errors"
	"ralph/internal/github"
	"ralph/internal/orchestrator"
	"ralph/internal/workspace"
)

// MockGitHubClient
type mockGitHub struct {
	issues    map[int]*github.Issue
	ciStatus  []github.CIStatus
	ciIndex   int
	logs      string
	prCreated string
}

func (m *mockGitHub) GetIssue(ctx context.Context, number int) (*github.Issue, error) {
	if iss, ok := m.issues[number]; ok {
		return iss, nil
	}
	return nil, fmt.Errorf("issue %d not found", number)
}

func (m *mockGitHub) CreatePR(ctx context.Context, title, bodyFile, baseBranch string) (string, error) {
	m.prCreated = title
	return "https://github.com/example/repo/pull/1", nil
}

func (m *mockGitHub) GetCIStatus(ctx context.Context, branch string) (github.CIStatus, error) {
	if m.ciIndex < len(m.ciStatus) {
		status := m.ciStatus[m.ciIndex]
		m.ciIndex++
		return status, nil
	}
	return github.CIStatusSuccess, nil
}

func (m *mockGitHub) GetFailedLogs(ctx context.Context, branch string) (string, error) {
	return m.logs, nil
}

// MockGitClient
type mockGit struct {
	branches []string
	pushed   []string
}

func (m *mockGit) CreateAndCheckoutBranch(ctx context.Context, branchName string) error {
	m.branches = append(m.branches, branchName)
	return nil
}

func (m *mockGit) Push(ctx context.Context, branchName string, force bool) error {
	m.pushed = append(m.pushed, branchName)
	return nil
}

func (m *mockGit) GetDiff(ctx context.Context, baseBranch string) (string, error) {
	return "", nil
}

// MockAgentRunner
type mockAgentRunner struct {
	onRun func(ctx context.Context, runID string, prompt string) error
}

func (m *mockAgentRunner) RunSession(ctx context.Context, runID string, prompt string) error {
	if m.onRun != nil {
		return m.onRun(ctx, runID, prompt)
	}
	return nil
}

// MockPromptAssembler
type mockPromptAssembler struct{}

func (m *mockPromptAssembler) BuildIssuePrompt(issue *github.Issue, specs map[string]string, handoff string) (string, error) {
	return "mock issue prompt", nil
}

func (m *mockPromptAssembler) BuildDraftPRPrompt(issues []*github.Issue, handoff string) (string, error) {
	return "mock draft pr prompt", nil
}

func (m *mockPromptAssembler) BuildCIFixPrompt(logs string, handoff string) (string, error) {
	return "mock ci fix prompt", nil
}

func setupTestEnv(t *testing.T) (string, *workspace.FSWorkspace) {
	t.Helper()
	tmpDir := t.TempDir()
	ws := workspace.NewFSWorkspace(tmpDir)
	ws.SetPollInterval(10 * time.Millisecond)
	return tmpDir, ws
}

func TestOrchestratorHappyPath(t *testing.T) {
	tmpDir, ws := setupTestEnv(t)

	gh := &mockGitHub{
		issues: map[int]*github.Issue{
			42: {Number: 42, Title: "Issue 42", Body: "Summary for 42"},
			45: {Number: 45, Title: "Issue 45", Body: "Summary for 45"},
		},
		ciStatus: []github.CIStatus{github.CIStatusSuccess},
	}
	gitClient := &mockGit{}
	prompts := &mockPromptAssembler{}

	agentRunner := &mockAgentRunner{
		onRun: func(ctx context.Context, runID string, prompt string) error {
			if runID == "draft-pr" {
				// Write .ralph/draft-pr.md
				draftDir := filepath.Join(tmpDir, ".ralph")
				_ = os.MkdirAll(draftDir, 0755)
				_ = os.WriteFile(filepath.Join(draftDir, "draft-pr.md"), []byte("# Unified Features\n\nCloses #42\nCloses #45"), 0644)
			}
			// Write done signal
			_ = os.WriteFile(filepath.Join(tmpDir, "ralph.done"), []byte("success"), 0644)
			return nil
		},
	}

	orch := orchestrator.New(gh, gitClient, agentRunner, ws, prompts, &orchestrator.Config{MaxRetries: 3})
	orch.SetRepoRoot(tmpDir)

	err := orch.Run(context.Background(), []int{42, 45})
	if err != nil {
		t.Fatalf("unexpected error in happy path: %v", err)
	}

	if len(gitClient.branches) != 1 || gitClient.branches[0] != "ralph/unified-42-45" {
		t.Errorf("unexpected branch created: %v", gitClient.branches)
	}
	if gh.prCreated != "Unified Features" {
		t.Errorf("expected PR title 'Unified Features', got %q", gh.prCreated)
	}
}

func TestOrchestratorEscalation(t *testing.T) {
	tmpDir, ws := setupTestEnv(t)

	gh := &mockGitHub{
		issues: map[int]*github.Issue{
			42: {Number: 42, Title: "Issue 42", Body: "Summary for 42"},
		},
	}
	gitClient := &mockGit{}
	prompts := &mockPromptAssembler{}

	agentRunner := &mockAgentRunner{
		onRun: func(ctx context.Context, runID string, prompt string) error {
			_ = os.WriteFile(filepath.Join(tmpDir, "ralph.escalate"), []byte("Missing database credentials"), 0644)
			return nil
		},
	}

	orch := orchestrator.New(gh, gitClient, agentRunner, ws, prompts, &orchestrator.Config{MaxRetries: 3})
	orch.SetRepoRoot(tmpDir)

	err := orch.Run(context.Background(), []int{42})
	if err == nil {
		t.Fatalf("expected ErrEscalation, got nil")
	}
	if !errors.Is(err, ralphErrors.ErrEscalation) {
		t.Errorf("expected ErrEscalation, got %v", err)
	}
	var esc *ralphErrors.EscalationError
	if errors.As(err, &esc) && esc.Reason != "Missing database credentials" {
		t.Errorf("expected blocker reason, got %q", esc.Reason)
	}
}

func TestOrchestratorCIFailureRetryAndSuccess(t *testing.T) {
	tmpDir, ws := setupTestEnv(t)

	gh := &mockGitHub{
		issues: map[int]*github.Issue{
			42: {Number: 42, Title: "Issue 42", Body: "Summary"},
		},
		ciStatus: []github.CIStatus{github.CIStatusFailure, github.CIStatusSuccess},
		logs:     "Tests failed on step 3",
	}
	gitClient := &mockGit{}
	prompts := &mockPromptAssembler{}

	var runs []string
	agentRunner := &mockAgentRunner{
		onRun: func(ctx context.Context, runID string, prompt string) error {
			runs = append(runs, runID)
			if runID == "draft-pr" {
				draftDir := filepath.Join(tmpDir, ".ralph")
				_ = os.MkdirAll(draftDir, 0755)
				_ = os.WriteFile(filepath.Join(draftDir, "draft-pr.md"), []byte("# PR Title\nCloses #42"), 0644)
			}
			_ = os.WriteFile(filepath.Join(tmpDir, "ralph.done"), []byte("success"), 0644)
			return nil
		},
	}

	orch := orchestrator.New(gh, gitClient, agentRunner, ws, prompts, &orchestrator.Config{MaxRetries: 3})
	orch.SetRepoRoot(tmpDir)

	err := orch.Run(context.Background(), []int{42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have executed issue-42, draft-pr, and ci-fix-1
	expectedRuns := []string{"issue-42", "draft-pr", "ci-fix-1"}
	if len(runs) != len(expectedRuns) {
		t.Fatalf("expected runs %v, got %v", expectedRuns, runs)
	}
	for i := range expectedRuns {
		if runs[i] != expectedRuns[i] {
			t.Errorf("run[%d] expected %s, got %s", i, expectedRuns[i], runs[i])
		}
	}
}

func TestOrchestratorCIFailureExhaustRetries(t *testing.T) {
	tmpDir, ws := setupTestEnv(t)

	gh := &mockGitHub{
		issues: map[int]*github.Issue{
			42: {Number: 42, Title: "Issue 42", Body: "Summary"},
		},
		ciStatus: []github.CIStatus{
			github.CIStatusFailure,
			github.CIStatusFailure,
			github.CIStatusFailure,
		},
		logs: "Persistent CI failure",
	}
	gitClient := &mockGit{}
	prompts := &mockPromptAssembler{}

	agentRunner := &mockAgentRunner{
		onRun: func(ctx context.Context, runID string, prompt string) error {
			if runID == "draft-pr" {
				draftDir := filepath.Join(tmpDir, ".ralph")
				_ = os.MkdirAll(draftDir, 0755)
				_ = os.WriteFile(filepath.Join(draftDir, "draft-pr.md"), []byte("# PR Title\nCloses #42"), 0644)
			}
			_ = os.WriteFile(filepath.Join(tmpDir, "ralph.done"), []byte("success"), 0644)
			return nil
		},
	}

	orch := orchestrator.New(gh, gitClient, agentRunner, ws, prompts, &orchestrator.Config{MaxRetries: 2})
	orch.SetRepoRoot(tmpDir)

	err := orch.Run(context.Background(), []int{42})
	if err == nil {
		t.Fatalf("expected ErrCIFailure, got nil")
	}
	if !errors.Is(err, ralphErrors.ErrCIFailure) {
		t.Errorf("expected ErrCIFailure, got %v", err)
	}
}
