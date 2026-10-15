package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"ralph/internal/config"

	"github.com/spf13/cobra"
)

type CheckResult struct {
	Name   string `json:"check"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

func NewValidateCmd() *cobra.Command {
	var jsonOutput bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and environment readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			var results []CheckResult

			// 1. Check AGENTS.md
			agentStat, err := os.Stat("AGENTS.md")
			if err != nil || agentStat.Size() == 0 {
				results = append(results, CheckResult{
					Name:   "AGENTS.md present and non-stub",
					Passed: false,
					Reason: "AGENTS.md not found or empty at repository root.",
				})
			} else {
				results = append(results, CheckResult{
					Name:   "AGENTS.md present and non-stub",
					Passed: true,
				})
			}

			// 2. Check ralph.json
			cfg, err := config.Load("")
			if err != nil {
				results = append(results, CheckResult{
					Name:   "ralph.json valid",
					Passed: false,
					Reason: fmt.Sprintf("Failed to load ralph.json: %v", err),
				})
			} else {
				results = append(results, CheckResult{
					Name:   "ralph.json valid",
					Passed: true,
				})
			}

			// 3. Check git CLI
			if _, err := exec.LookPath("git"); err != nil {
				results = append(results, CheckResult{
					Name:   "git CLI available",
					Passed: false,
					Reason: "git binary not found in PATH",
				})
			} else {
				results = append(results, CheckResult{
					Name:   "git CLI available",
					Passed: true,
				})
			}

			// 4. Check gh CLI
			if _, err := exec.LookPath("gh"); err != nil {
				results = append(results, CheckResult{
					Name:   "gh CLI available",
					Passed: false,
					Reason: "gh binary not found in PATH",
				})
			} else {
				results = append(results, CheckResult{
					Name:   "gh CLI available",
					Passed: true,
				})
			}

			// 5. Check Agent CLI (e.g. codex)
			agentCmd := "codex"
			if cfg != nil && cfg.Agent.Codex.Command != "" {
				agentCmd = cfg.Agent.Codex.Command
			}
			if _, err := exec.LookPath(agentCmd); err != nil {
				results = append(results, CheckResult{
					Name:   fmt.Sprintf("Agent CLI (%s) available", agentCmd),
					Passed: false,
					Reason: fmt.Sprintf("%s binary not found in PATH", agentCmd),
				})
			} else {
				results = append(results, CheckResult{
					Name:   fmt.Sprintf("Agent CLI (%s) available", agentCmd),
					Passed: true,
				})
			}

			// 6. Check lockfile
			if _, err := os.Stat(".ralph-lock"); err == nil {
				results = append(results, CheckResult{
					Name:   "No active lockfile",
					Passed: false,
					Reason: ".ralph-lock is present. Check if another instance is running.",
				})
			} else {
				results = append(results, CheckResult{
					Name:   "No active lockfile",
					Passed: true,
				})
			}

			hasFailure := false
			for _, r := range results {
				if !r.Passed {
					hasFailure = true
				}
			}

			if jsonOutput {
				bytes, _ := json.MarshalIndent(results, "", "  ")
				fmt.Println(string(bytes))
			} else {
				for _, r := range results {
					if r.Passed {
						if !quiet {
							fmt.Printf("✓ %s\n", r.Name)
						}
					} else {
						fmt.Printf("✗ %s\n", r.Name)
						if r.Reason != "" {
							fmt.Printf("    %s\n", r.Reason)
						}
					}
				}
			}

			if hasFailure {
				return fmt.Errorf("system validation failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Print only failures")

	return cmd
}
