package agent

import (
	"context"
)

type AgentRunner interface {
	RunSession(ctx context.Context, runID string, prompt string) error
}
