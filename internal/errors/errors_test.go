package errors_test

import (
	"errors"
	"fmt"
	"testing"

	ralphErrors "ralph/internal/errors"
)

func TestErrorTaxonomy(t *testing.T) {
	sysErr := ralphErrors.NewSystemError(fmt.Errorf("missing config"))
	if !errors.Is(sysErr, ralphErrors.ErrSystem) {
		t.Fatalf("expected sysErr to match ErrSystem")
	}

	escErr := ralphErrors.NewEscalationError("missing required spec")
	if !errors.Is(escErr, ralphErrors.ErrEscalation) {
		t.Fatalf("expected escErr to match ErrEscalation")
	}
	var targetEsc *ralphErrors.EscalationError
	if !errors.As(escErr, &targetEsc) {
		t.Fatalf("expected escErr to be unwrapable as *EscalationError")
	}
	if targetEsc.Reason != "missing required spec" {
		t.Fatalf("expected reason 'missing required spec', got %q", targetEsc.Reason)
	}

	ciErr := ralphErrors.NewCIFailureError(3, "build failed on step test")
	if !errors.Is(ciErr, ralphErrors.ErrCIFailure) {
		t.Fatalf("expected ciErr to match ErrCIFailure")
	}
	var targetCI *ralphErrors.CIFailureError
	if !errors.As(ciErr, &targetCI) {
		t.Fatalf("expected ciErr to be unwrapable as *CIFailureError")
	}
	if targetCI.Retries != 3 || targetCI.Logs != "build failed on step test" {
		t.Fatalf("unexpected CI error fields: %+v", targetCI)
	}
}
