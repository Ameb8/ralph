package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ralph",
		Short: "Ralph automated issue-driven development system",
	}

	cmd.AddCommand(NewRunCmd())
	cmd.AddCommand(NewValidateCmd())

	return cmd
}

func Execute(ctx context.Context) error {
	cmd := NewRootCmd()
	cmd.SetContext(ctx)
	return cmd.Execute()
}
