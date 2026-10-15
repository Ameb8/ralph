package errors

import (
	"errors"
	"fmt"
)

var (
	// ErrSystem represents internal failures like missing configurations, bad CLI flags, missing binaries.
	ErrSystem = errors.New("system error")

	// ErrEscalation represents explicit blocker signaled by the agent via ralph.escalate.
	ErrEscalation = errors.New("agent escalation")

	// ErrCIFailure represents CI check failures exhausting all max_retries.
	ErrCIFailure = errors.New("ci failure")
)

// EscalationError captures the agent's blocker description.
type EscalationError struct {
	Reason string
}

func (e *EscalationError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("agent escalated: %s", e.Reason)
	}
	return "agent escalated"
}

func (e *EscalationError) Is(target error) bool {
	return target == ErrEscalation
}

// CIFailureError captures failure when max retries are exhausted.
type CIFailureError struct {
	Retries int
	Logs    string
}

func (e *CIFailureError) Error() string {
	return fmt.Sprintf("CI failed after %d retries", e.Retries)
}

func (e *CIFailureError) Is(target error) bool {
	return target == ErrCIFailure
}

// SystemError wraps system-level failures.
type SystemError struct {
	Err error
}

func (e *SystemError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("system error: %v", e.Err)
	}
	return "system error"
}

func (e *SystemError) Unwrap() error {
	return e.Err
}

func (e *SystemError) Is(target error) bool {
	return target == ErrSystem
}

// NewSystemError wraps an underlying error as ErrSystem.
func NewSystemError(err error) error {
	return &SystemError{Err: err}
}

// NewEscalationError creates an ErrEscalation with the blocker reason.
func NewEscalationError(reason string) error {
	return &EscalationError{Reason: reason}
}

// NewCIFailureError creates an ErrCIFailure after exhausting retries.
func NewCIFailureError(retries int, logs string) error {
	return &CIFailureError{Retries: retries, Logs: logs}
}
