package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CLIClient implements GitClient by executing the git CLI.
type CLIClient struct {
	dir string
}

// NewCLIClient creates a new GitClient for the given directory.
func NewCLIClient(dir string) *CLIClient {
	if dir == "" {
		dir = "."
	}
	return &CLIClient{dir: dir}
}

// CreateAndCheckoutBranch creates and switches to a new branch.
// If the branch already exists, it checks it out.
func (c *CLIClient) CreateAndCheckoutBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	cmd.Dir = c.dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "already exists") {
			checkoutCmd := exec.CommandContext(ctx, "git", "checkout", branchName)
			checkoutCmd.Dir = c.dir
			var checkoutErr bytes.Buffer
			checkoutCmd.Stderr = &checkoutErr
			if err2 := checkoutCmd.Run(); err2 != nil {
				return fmt.Errorf("git checkout %s failed: %s (%w)", branchName, checkoutErr.String(), err2)
			}
			return nil
		}
		return fmt.Errorf("git checkout -b %s failed: %s (%w)", branchName, stderr.String(), err)
	}
	return nil
}

// Push pushes the branch to origin.
func (c *CLIClient) Push(ctx context.Context, branchName string, force bool) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	args = append(args, "-u", "origin", branchName)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = c.dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %s (%w)", stderr.String(), err)
	}
	return nil
}

// GetDiff gets the git diff against a base branch.
func (c *CLIClient) GetDiff(ctx context.Context, baseBranch string) (string, error) {
	target := baseBranch
	if target == "" {
		target = "HEAD"
	}
	cmd := exec.CommandContext(ctx, "git", "diff", target)
	cmd.Dir = c.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff %s failed: %s (%w)", target, stderr.String(), err)
	}
	return stdout.String(), nil
}
