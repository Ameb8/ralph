# Ralph Architecture

This document details the high-level module structure and interface contracts for Ralph. It is intended for contributors and maintainers of the `ralph` CLI.

## 1. Repository Structure

Ralph is built in Go and strictly separates side-effects (shelling out to CLIs, reading files) from its core orchestration logic.

```text
ralph/
├── cmd/
│   └── ralph/                # Entry point (main.go); sets up Cobra CLI
├── internal/
│   ├── cli/                  # Cobra command definitions (root, run, validate)
│   ├── orchestrator/         # Deep module: executes Phase A, B, C pipelines
│   ├── workspace/            # Deep module: local file state, lockfiles, branching, handoff
│   ├── prompt/               # Prompt assembly engine (using text/template)
│   ├── agent/                # Adapter: executes the AI agent (pi, codex)
│   ├── github/               # Adapter: shells out to `gh` CLI
│   └── git/                  # Adapter: shells out to `git` CLI
├── templates/                # //go:embed templates for prompt assembly
├── ralph.json                # Default system config
├── go.mod
└── go.sum
```

## 2. Core Interfaces & Seams

To ensure high **locality** for maintainers and maximum **testability**, the system is designed around several "seams". External side-effects are hidden behind deep adapters, allowing the complex pipeline logic to be tested purely in-memory.

### 2.1 External Adapters

Instead of scattering `os/exec` calls throughout the codebase, we constrain them to specific interface implementations.

**GitHub Seam (`internal/github`)**  
Hides the `gh` CLI execution, JSON unmarshaling, and polling logic.

```go
type GitHubClient interface {
    GetIssue(ctx context.Context, number int) (*Issue, error)
    CreatePR(ctx context.Context, title, bodyFile, baseBranch string) (string, error)
    GetCIStatus(ctx context.Context, branch string) (CIStatus, error)
    GetFailedLogs(ctx context.Context, branch string) (string, error)
}
```

**Git Seam (`internal/git`)**  
Hides `git` commands and branch state management.

```go
type GitClient interface {
    CreateAndCheckoutBranch(ctx context.Context, branchName string) error
    Push(ctx context.Context, branchName string, force bool) error
    GetDiff(ctx context.Context, baseBranch string) (string, error)
}
```

**Agent Seam (`internal/agent`)**  
Hides the specifics of invoking the configured AI agent. It handles prompt delivery modes (`arg`, `stdin`, `file`), manages the subprocess lifecycle, and captures stdout/stderr into `.ralph/runs/<run_id>/` for debugging.

```go
type AgentRunner interface {
    // RunSession blocks until the agent exits. Returns an error if the agent
    // exits non-zero, allowing the host to treat it as an escalation.
    RunSession(ctx context.Context, runID string, prompt string) error
}
```

### 2.2 Internal State Modules

**Workspace Module (`internal/workspace`)**  
Provides a deep implementation for repository state tracking. It abstracts the file system, managing lockfiles, polling for file IPC signals (`ralph.done`, `ralph.escalate`), and tracking the `.ralph-handoff.md` context.

```go
type SignalType string
const (
    SignalDone     SignalType = "done"
    SignalEscalate SignalType = "escalate"
)

type Workspace interface {
    AcquireLock(issueNums []int) error
    ReleaseLock() error
    
    // Handoff management
    InitializeHandoff() error
    ReadHandoff() (string, error)
    
    // File IPC Polling
    WaitForSignal(ctx context.Context) (SignalType, error)
    
    // Spec extraction
    ExtractSpecs(repoRoot string, specPaths []string) (map[string]string, error)
}
```

**Prompt Engine (`internal/prompt`)**  
Provides pure functions that assemble markdown by combining `templates/*.tmpl` files with GitHub issue data, loaded spec contents, and handoff context.

```go
type Assembler interface {
    BuildIssuePrompt(issue *Issue, specs map[string]string, handoff string) (string, error)
    BuildDraftPRPrompt(issues []*Issue, handoff string) (string, error)
    BuildCIFixPrompt(logs string, handoff string) (string, error)
}
```

### 2.3 The Orchestrator

The `Orchestrator` (`internal/orchestrator`) is the brain of the system. It has almost no public interface. It is instantiated with the adapters above and executes the Phase A, B, and C pipelines defined in the specification.

```go
type Orchestrator struct {
    github    GitHubClient
    git       GitClient
    agent     AgentRunner
    workspace Workspace
    prompts   Assembler
    config    *Config
}

// Run executes the unified PR vertical slice.
func (o *Orchestrator) Run(ctx context.Context, issueNumbers []int) error
```

## 3. Design Principles

- **Cancellation & Lifecycle:** Every interface method that blocks or executes a subprocess accepts a `context.Context`. This ensures that an OS interrupt (`Ctrl+C`) caught at the CLI layer cleanly cascades down, killing `gh`, `git`, and `agent` subprocesses safely before exiting.
- **Errors as Control Flow:** The system relies on typed custom errors (`ErrEscalation`, `ErrCIFailure`, `ErrSystem`) to manage control flow. The Orchestrator inspects these errors to determine whether to exit gracefully (e.g., exiting 1 on agent escalation without a stack trace) or crash (e.g., missing binaries or malformed config).
- **Test-Driven Design:** Because the `Orchestrator` operates entirely against interfaces, the complex retry loops and branching logic can be tested in milliseconds using in-memory Fakes and Mocks (e.g., simulating a CI failure on the first pass and a success on the second pass without ever spinning up a real LLM or hitting the network).