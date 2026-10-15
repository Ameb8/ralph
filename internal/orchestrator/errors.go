package orchestrator

import "errors"

var (
	ErrEscalation = errors.New("agent escalated")
	ErrCIFailure  = errors.New("ci failure")
	ErrSystem     = errors.New("system error")
)
