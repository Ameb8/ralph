package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run [issues...]",
		Short: "Run the Ralph pipeline for a set of issues",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var issueNumbers []int
			for _, arg := range args {
				num, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid issue number: %s", arg)
				}
				issueNumbers = append(issueNumbers, num)
			}

			// TODO: Initialize orchestrator and dependencies here
			fmt.Printf("Working on Issue #%d... (and %d others)\n", issueNumbers[0], len(issueNumbers)-1)
			return nil
		},
	}
}
