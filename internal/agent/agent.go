package agent

import (
	"context"
)

// AgentRunner defines the contract for executing an autonomous coding agent.
type AgentRunner interface {
	// RunSession executes a single agent session non-interactively with the provided prompt.
	// It captures stdout/stderr to .ralph/runs/<runID>/agent.log, respects context cancellation,
	// and returns an error if the agent process exits non-zero or fails.
	RunSession(ctx context.Context, runID string, prompt string) error
}
