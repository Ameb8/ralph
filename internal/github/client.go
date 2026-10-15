package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// CLIClient implements GitHubClient by executing the `gh` CLI.
type CLIClient struct {
	dir string
}

// NewCLIClient creates a new GitHubClient.
func NewCLIClient(dir string) *CLIClient {
	if dir == "" {
		dir = "."
	}
	return &CLIClient{dir: dir}
}

// GetIssue fetches an issue's details by number.
func (c *CLIClient) GetIssue(ctx context.Context, number int) (*Issue, error) {
	cmd := exec.CommandContext(ctx, "gh", "issue", "view", strconv.Itoa(number), "--json", "number,title,body")
	cmd.Dir = c.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh issue view %d failed: %s (%w)", number, stderr.String(), err)
	}

	var issue Issue
	if err := json.Unmarshal(stdout.Bytes(), &issue); err != nil {
		return nil, fmt.Errorf("failed to parse issue JSON: %w", err)
	}

	return &issue, nil
}

// CreatePR creates a pull request via gh pr create.
func (c *CLIClient) CreatePR(ctx context.Context, title, bodyFile, baseBranch string) (string, error) {
	args := []string{"pr", "create", "--title", title, "--body-file", bodyFile}
	if baseBranch != "" {
		args = append(args, "--base", baseBranch)
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = c.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh pr create failed: %s (%w)", stderr.String(), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetCIStatus polls CI checks for the given branch using gh pr checks --watch.
func (c *CLIClient) GetCIStatus(ctx context.Context, branch string) (CIStatus, error) {
	args := []string{"pr", "checks"}
	if branch != "" {
		args = append(args, branch)
	}
	args = append(args, "--watch")

	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = c.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if err == nil {
		return CIStatusSuccess, nil
	}

	return CIStatusFailure, nil
}

// GetFailedLogs fetches the failure logs for the most recent run on the branch.
func (c *CLIClient) GetFailedLogs(ctx context.Context, branch string) (string, error) {
	// First attempt to get the latest run ID for this branch
	listArgs := []string{"run", "list"}
	if branch != "" {
		listArgs = append(listArgs, "--branch", branch)
	}
	listArgs = append(listArgs, "--limit", "1", "--json", "databaseId")

	listCmd := exec.CommandContext(ctx, "gh", listArgs...)
	listCmd.Dir = c.dir
	var listStdout bytes.Buffer
	listCmd.Stdout = &listStdout

	var runID string
	if err := listCmd.Run(); err == nil {
		type runEntry struct {
			DatabaseID int64 `json:"databaseId"`
		}
		var runs []runEntry
		if err := json.Unmarshal(listStdout.Bytes(), &runs); err == nil && len(runs) > 0 && runs[0].DatabaseID > 0 {
			runID = strconv.FormatInt(runs[0].DatabaseID, 10)
		}
	}

	viewArgs := []string{"run", "view"}
	if runID != "" {
		viewArgs = append(viewArgs, runID)
	}
	viewArgs = append(viewArgs, "--log-failed")

	viewCmd := exec.CommandContext(ctx, "gh", viewArgs...)
	viewCmd.Dir = c.dir
	var viewStdout, viewStderr bytes.Buffer
	viewCmd.Stdout = &viewStdout
	viewCmd.Stderr = &viewStderr

	if err := viewCmd.Run(); err != nil {
		// If failed with specific run ID, fallback to bare gh run view --log-failed
		if runID != "" {
			fallbackCmd := exec.CommandContext(ctx, "gh", "run", "view", "--log-failed")
			fallbackCmd.Dir = c.dir
			var fallbackStdout, fallbackStderr bytes.Buffer
			fallbackCmd.Stdout = &fallbackStdout
			fallbackCmd.Stderr = &fallbackStderr
			if err2 := fallbackCmd.Run(); err2 == nil && fallbackStdout.Len() > 0 {
				return fallbackStdout.String(), nil
			}
		}
		return "", fmt.Errorf("gh run view --log-failed failed: %s (%w)", viewStderr.String(), err)
	}

	return viewStdout.String(), nil
}
