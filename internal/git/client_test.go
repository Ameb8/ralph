package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ralph/internal/git"
)

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, string(out))
		}
	}

	run("init")
	run("config", "user.name", "Test User")
	run("config", "user.email", "test@example.com")

	// Create initial commit
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Initial"), 0644); err != nil {
		t.Fatalf("failed to write initial file: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "chore: initial commit")

	return dir
}

func TestCLIClientBranchAndDiff(t *testing.T) {
	dir := initTestGitRepo(t)
	client := git.NewCLIClient(dir)
	ctx := context.Background()

	branchName := "ralph/unified-42-45"
	if err := client.CreateAndCheckoutBranch(ctx, branchName); err != nil {
		t.Fatalf("failed to create and checkout branch: %v", err)
	}

	// Calling again should successfully checkout existing branch
	if err := client.CreateAndCheckoutBranch(ctx, branchName); err != nil {
		t.Fatalf("failed to checkout already existing branch: %v", err)
	}

	// Modify file and test diff
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Updated README"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	diff, err := client.GetDiff(ctx, "HEAD")
	if err != nil {
		t.Fatalf("failed to get diff: %v", err)
	}
	if !strings.Contains(diff, "Updated README") {
		t.Errorf("expected diff to contain 'Updated README', got %q", diff)
	}
}
