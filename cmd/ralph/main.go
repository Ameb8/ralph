package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"ralph/internal/cli"
	ralphErrors "ralph/internal/errors"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := cli.Execute(ctx); err != nil {
		var escErr *ralphErrors.EscalationError
		var ciErr *ralphErrors.CIFailureError

		if errors.As(err, &escErr) {
			fmt.Fprintf(os.Stderr, "Agent Escalation: %s\n", escErr.Reason)
			os.Exit(1)
		} else if errors.As(err, &ciErr) {
			fmt.Fprintf(os.Stderr, "CI Failure: %s\n", ciErr.Error())
			if ciErr.Logs != "" {
				fmt.Fprintf(os.Stderr, "Failure Logs:\n%s\n", ciErr.Logs)
			}
			os.Exit(1)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
