# Commit Plan: main

Generated for current untracked, unstaged, and staged changes, explicitly limited to the files modified in the current conversation.

## Commit 1: Initialize CLI structure

Files:
- `cmd/ralph/main.go`
- `internal/cli/root.go`
- `internal/cli/run.go`
- `internal/cli/validate.go`
- `go.mod`
- `go.sum`

Rationale:
This commit establishes the foundational entrypoint and Cobra CLI structure for the Ralph command-line interface. It includes signal wiring for graceful termination and defines the root, run, and validate commands. `go.mod` and `go.sum` are included as dependencies were tidied.

Git add:
```bash
git add -- cmd/ralph/main.go internal/cli/root.go internal/cli/run.go internal/cli/validate.go go.mod go.sum
```

Commit message:
```text
feat(cli): initialize cobra command structure and entrypoint

Establishes the foundational execution path for the Ralph CLI, including context signal management for graceful degradation upon interrupt. Configures the base Cobra setup with a root command and functional stubs for the `run` and `validate` subcommands, satisfying the primary V0.1.0 usage patterns.

Co-Authored-By: OpenAI GPT-5.6-Terra <noreply@openai.com>
```

## Commit 2: Define orchestrator interfaces and state modules

Files:
- `internal/github/github.go`
- `internal/git/git.go`
- `internal/agent/agent.go`
- `internal/workspace/workspace.go`
- `internal/prompt/prompt.go`
- `internal/orchestrator/orchestrator.go`

Rationale:
This encapsulates the core business logic boundaries as defined in the architectural specifications. It creates deep adapters for Git, GitHub, agent subprocess execution, workspace management, and prompt assembly to enable test-driven orchestration without directly triggering real AI sessions or network side-effects.

Git add:
```bash
git add -- internal/github/github.go internal/git/git.go internal/agent/agent.go internal/workspace/workspace.go internal/prompt/prompt.go internal/orchestrator/orchestrator.go
```

Commit message:
```text
feat(orchestrator): define core module interfaces and system boundaries

Implements the structural seams required to build the Ralph unified PR pipeline with complete testability. Defines adapter interfaces for GitHub (`gh`), Git, subprocess agents, and local workspace locking. Introduces the base Orchestrator structure which will manage the sequencing of these isolated contexts.

Co-Authored-By: OpenAI GPT-5.6-Terra <noreply@openai.com>
```

## Commit 3: Add basic configuration and documentation

Files:
- `ralph.json`
- `templates/.gitkeep`
- `README.md`

Rationale:
Completes the base scaffold by providing a minimal set of instructions for building and running the project, initializing the template directory, and providing the default `ralph.json` configuration file as expected by the V0.1.0 specifications.

Git add:
```bash
git add -- ralph.json templates/.gitkeep README.md
```

Commit message:
```text
docs(readme): provide minimal configuration and build instructions

Adds the baseline `ralph.json` file configuring `pi` as the default agent and establishing retry parameters. Creates a basic README with Go build instructions, and stubs out the `templates/` directory required for future prompt generation.

Co-Authored-By: OpenAI GPT-5.6-Terra <noreply@openai.com>
```

## Coverage Check

- `cmd/ralph/main.go` - Commit 1
- `internal/cli/root.go` - Commit 1
- `internal/cli/run.go` - Commit 1
- `internal/cli/validate.go` - Commit 1
- `go.mod` - Commit 1
- `go.sum` - Commit 1
- `internal/github/github.go` - Commit 2
- `internal/git/git.go` - Commit 2
- `internal/agent/agent.go` - Commit 2
- `internal/workspace/workspace.go` - Commit 2
- `internal/prompt/prompt.go` - Commit 2
- `internal/orchestrator/orchestrator.go` - Commit 2
- `ralph.json` - Commit 3
- `templates/.gitkeep` - Commit 3
- `README.md` - Commit 3
