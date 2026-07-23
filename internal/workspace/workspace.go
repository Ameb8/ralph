package workspace

import (
	"context"
)

type SignalType string

const (
	SignalDone     SignalType = "done"
	SignalEscalate SignalType = "escalate"
)

type Workspace interface {
	AcquireLock(issueNums []int) error
	ReleaseLock() error

	InitializeHandoff() error
	ReadHandoff() (string, error)

	WaitForSignal(ctx context.Context) (SignalType, error)

	ExtractSpecs(repoRoot string, specPaths []string) (map[string]string, error)
}
