package cli

import (
	"fmt"
	"strconv"

	"ralph/internal/agent"
	"ralph/internal/config"
	ralphErrors "ralph/internal/errors"
	"ralph/internal/git"
	"ralph/internal/github"
	"ralph/internal/orchestrator"
	"ralph/internal/prompt"
	"ralph/internal/workspace"

	"github.com/spf13/cobra"
)

func NewRunCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "run [issues...]",
		Short: "Run the Ralph pipeline for a set of issues",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var issueNumbers []int
			for _, arg := range args {
				num, err := strconv.Atoi(arg)
				if err != nil {
					return ralphErrors.NewSystemError(fmt.Errorf("invalid issue number %q: %w", arg, err))
				}
				issueNumbers = append(issueNumbers, num)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}

			githubClient := github.NewCLIClient(".")
			gitClient := git.NewCLIClient(".")

			agentRunner, err := agent.NewRunner(&cfg.Agent)
			if err != nil {
				return ralphErrors.NewSystemError(err)
			}

			ws := workspace.NewFSWorkspace(".")
			assembler, err := prompt.New()
			if err != nil {
				return ralphErrors.NewSystemError(err)
			}

			orch := orchestrator.New(
				githubClient,
				gitClient,
				agentRunner,
				ws,
				assembler,
				&orchestrator.Config{MaxRetries: cfg.MaxRetries},
			)

			return orch.Run(cmd.Context(), issueNumbers)
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to configuration file (defaults to ralph.json)")

	return cmd
}
