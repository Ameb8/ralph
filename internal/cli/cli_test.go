package cli_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"ralph/internal/cli"
)

func TestRootCommand(t *testing.T) {
	cmd := cli.NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing root help: %v", err)
	}

	if !strings.Contains(buf.String(), "ralph") {
		t.Errorf("expected help output to contain 'ralph', got %s", buf.String())
	}
}

func TestRunCommandArgsValidation(t *testing.T) {
	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when no issues provided to run, got nil")
	}

	cmd = cli.NewRootCmd()
	cmd.SetArgs([]string{"run", "not-a-number"})
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected error when invalid issue number provided, got nil")
	}
	if !strings.Contains(err.Error(), "invalid issue number") {
		t.Errorf("expected error message to mention invalid issue number, got %v", err)
	}
}

func TestValidateCommand(t *testing.T) {
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	if err := os.Chdir("../.."); err != nil {
		t.Fatalf("failed to chdir to repo root: %v", err)
	}
	defer func() {
		_ = os.Chdir(origWd)
	}()

	cmd := cli.NewRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"validate"})

	ctx := context.Background()
	cmd.SetContext(ctx)

	err = cmd.Execute()
	if err != nil {
		t.Logf("validate output:\n%s", buf.String())
		t.Fatalf("expected validate to succeed in repo root: %v", err)
	}
}
