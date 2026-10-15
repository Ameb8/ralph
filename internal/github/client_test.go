package github_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"ralph/internal/github"
)

func TestGitHubCLIClient(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	mockGH := filepath.Join(binDir, "gh")
	mockScript := `#!/bin/sh
case "$1" in
  issue)
    if [ "$2" = "view" ] && [ "$3" = "42" ]; then
      cat << 'EOF'
{"number": 42, "title": "Test Issue", "body": "## Summary\nTest body\n\n## Specs\n- docs/specs/test.md"}
EOF
      exit 0
    fi
    ;;
  pr)
    if [ "$2" = "create" ]; then
      echo "https://github.com/example/repo/pull/100"
      exit 0
    elif [ "$2" = "checks" ]; then
      if [ "$3" = "fail-branch" ]; then
        exit 1
      fi
      exit 0
    fi
    ;;
  run)
    if [ "$2" = "list" ]; then
      echo '[{"databaseId": 999}]'
      exit 0
    elif [ "$2" = "view" ]; then
      echo "Step 'Run tests' failed with exit code 1"
      exit 0
    fi
    ;;
esac
exit 1
`
	if err := os.WriteFile(mockGH, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to write mock gh: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(filepath.ListSeparator)+origPath)

	client := github.NewCLIClient(tmpDir)
	ctx := context.Background()

	// Test GetIssue
	issue, err := client.GetIssue(ctx, 42)
	if err != nil {
		t.Fatalf("failed to get issue: %v", err)
	}
	if issue.Number != 42 || issue.Title != "Test Issue" {
		t.Errorf("unexpected issue content: %+v", issue)
	}

	// Test CreatePR
	prURL, err := client.CreatePR(ctx, "Test PR", "draft.md", "main")
	if err != nil {
		t.Fatalf("failed to create PR: %v", err)
	}
	if prURL != "https://github.com/example/repo/pull/100" {
		t.Errorf("expected PR URL, got %q", prURL)
	}

	// Test GetCIStatus success
	status, err := client.GetCIStatus(ctx, "success-branch")
	if err != nil {
		t.Fatalf("failed to get CI status: %v", err)
	}
	if status != github.CIStatusSuccess {
		t.Errorf("expected success status, got %q", status)
	}

	// Test GetCIStatus failure
	status, err = client.GetCIStatus(ctx, "fail-branch")
	if err != nil {
		t.Fatalf("unexpected error on failing checks: %v", err)
	}
	if status != github.CIStatusFailure {
		t.Errorf("expected failure status, got %q", status)
	}

	// Test GetFailedLogs
	logs, err := client.GetFailedLogs(ctx, "fail-branch")
	if err != nil {
		t.Fatalf("failed to get failed logs: %v", err)
	}
	if logs != "Step 'Run tests' failed with exit code 1\n" {
		t.Errorf("unexpected failed logs: %q", logs)
	}
}
