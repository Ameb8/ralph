package orchestrator

import (
	"context"
	"fmt"

	"ralph/internal/agent"
	"ralph/internal/git"
	"ralph/internal/github"
	"ralph/internal/prompt"
	"ralph/internal/workspace"
)

type Config struct {
	MaxRetries int
}

type Orchestrator struct {
	github    github.GitHubClient
	git       git.GitClient
	agent     agent.AgentRunner
	workspace workspace.Workspace
	prompts   prompt.Assembler
	config    *Config
}

func New(
	githubClient github.GitHubClient,
	gitClient git.GitClient,
	agentRunner agent.AgentRunner,
	ws workspace.Workspace,
	assembler prompt.Assembler,
	cfg *Config,
) *Orchestrator {
	return &Orchestrator{
		github:    githubClient,
		git:       gitClient,
		agent:     agentRunner,
		workspace: ws,
		prompts:   assembler,
		config:    cfg,
	}
}

func (o *Orchestrator) Run(ctx context.Context, issueNumbers []int) error {
	// TODO: Implement Phase A, B, C pipelines
	fmt.Println("Orchestrator run started...")
	return nil
}
