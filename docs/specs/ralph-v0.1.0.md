# Ralph V0.1.0 — Unified PR Vertical Slice
## Implementation Specification

---

## 1. Purpose & Scope

This specification defines **V0.1.0** of the Ralph automated issue-driven development system. It represents the first fully functional vertical slice of the architecture defined in `docs/specs/ralph.md`. 

Rather than running as a background daemon processing a queue of labeled issues, V0.1.0 operates as a synchronous CLI tool that takes a specific list of GitHub issue numbers as arguments. It orchestrates the implementation of these issues and produces one **unified Pull Request**.

Crucially, to maintain high-quality agent focus, V0.1.0 uses **fresh agent context windows for each distinct phase of work**:
1. A fresh session for *each individual issue* in the batch.
2. A fresh session dedicated solely to *drafting the PR*.
3. A fresh session for *fixing CI failures*, if they occur.

### 1.1 In Scope for V0.1.0
- **CLI Entrypoint:** Synchronous execution via `ralph run <issue_1> <issue_2> ...`.
- **Sequential Isolated Contexts:** Executing a fresh agent session for each issue, passing only the relevant issue body, its specs, and a lightweight handoff document from previous sessions.
- **Context Handoff:** A mechanism (`.ralph-handoff.md`) for agents to leave notes for subsequent agent sessions about what was built or changed.
- **Dedicated PR Drafting:** A final, isolated agent session that reads the handoff context and issues to write the unified PR body.
- **CI Loop:** Polling GitHub Actions for CI status and executing a retry loop (spawning a fresh agent session with failure logs) up to `max_retries`.
- **Codex CLI Agent Backend:** Initial concrete agent implementation using OpenAI Codex CLI (`codex exec`), abstracted behind a clean runner interface.

### 1.2 Out of Scope for V0.1.0
- **Queue Management:** No polling for `ralph-ready` labels.
- **Dependency Resolution:** The `Depends On` issue relationships are ignored; the user manually ensures the provided issues are ready.
- **Scope Enforcement (Diff checking):** Bypassed for V0.1.0.
- **GitHub Action PR Trigger:** Instead of a separate `.github/workflows/draft-pr.yml` workflow, the V0.1.0 host script will directly execute `gh pr create` to simplify the initial infrastructure footprint.
- **Verbose Output & Live Logs:** A `--verbose` flag or streaming live agent stdout/stderr to the user's terminal is out of scope.
- **Alternative Agent Backends:** Implementing additional coding agents (e.g., Claude Code, Pi, Aider) in V0.1.0. Only Codex CLI is supported in V0.1.0, though the agent abstraction is designed for easy future extension.

---

## 2. System Workflow

The V0.1.0 execution follows a strict pipeline divided into three distinct phases.

### Phase A: Sequential Issue Implementation
1. **Invocation:** Operator runs `ralph run 42 45`
2. **Branching:** Host creates and checks out a new branch: `ralph/unified-42-45`
3. **Initialization:** Host creates an empty `.ralph-handoff.md` file to track inter-session context.
4. **Issue Iteration:** For *each* issue in the provided list:
   - Host queries GitHub for the issue body and extracts its referenced `docs/specs/*.md` files.
   - Host builds a prompt combining:
     - The current `.ralph-handoff.md` (what previous sessions did)
     - The specific Issue body and its Specs
     *(Note: `AGENTS.md` is deliberately NOT embedded in the prompt. Codex CLI natively reads repo-level instruction files like `AGENTS.md`, avoiding token duplication).*
   - Host spawns a **fresh agent session** via the `AgentRunner` interface (invoking `codex exec`).
   - Agent implements the issue, commits chunks, and appends a summary of its work to `.ralph-handoff.md`.
   - Agent signals completion via `ralph.done`. Host terminates this session and moves to the next issue.
   - *Escalation Exception:* If `ralph.escalate` is detected, the host prints the blocker and exits 1 immediately.

### Phase B: PR Drafting
1. **Context Assembly:** After all issues are completed, the host builds a new prompt containing:
   - All target Issue bodies.
   - The final `.ralph-handoff.md` (the summary of all implemented work).
   - Explicit instructions directing the agent to execute the repo's Draft-PR skill (defined in `AGENTS.md`).
2. **PR Agent Session:** Host spawns a **fresh agent session** tasked *only* with drafting the PR.
3. **Signal & Open:** Agent writes `.ralph/draft-pr.md`, pushes the branch, and writes `ralph.done`. The host reads the draft file and executes `gh pr create`.

### Phase C: CI Polling & Retry Loop
1. **CI Polling:** Host polls `gh pr checks --watch`. If green, the host exits 0.
2. **Failure Detection:** If red, the host fetches failure logs (`gh run view --log-failed`).
3. **CI Fix Session:** Host builds a new prompt containing:
   - The CI failure logs.
   - The `.ralph-handoff.md` (so the agent knows what the architecture looks like).
   - Instructions to fix the build, commit, force-push, and signal `ralph.done`.
4. **Retry Run:** Host spawns a **fresh agent session** with this prompt. Once `ralph.done` is written, the host returns to Step 1. Repeats up to `max_retries`.

---

## 4. Agent Invocation System

For V0.1.0, Ralph exclusively implements and uses the **OpenAI Codex CLI** (`codex exec`) as its initial agent backend. However, to ensure that adding other agent options in the future (e.g., Claude Code, Pi, Aider) is not overly difficult, the system isolates all agent execution behind a clean **`AgentRunner` abstraction seam and factory pattern**. Core orchestration loops interact solely with this abstraction and remain completely agnostic to CLI-specific arguments, process semantics, or flags.

### 4.1 Architecture & Abstraction Seam

The orchestrator has zero knowledge of `codex` CLI specifics. It delegates all agent execution to the `AgentRunner` interface:

```go
package agent

import "context"

// AgentRunner defines the contract for executing an autonomous coding agent.
type AgentRunner interface {
    // RunSession executes a single agent session non-interactively with the provided prompt.
    // It captures stdout/stderr to .ralph/runs/<runID>/agent.log, respects context cancellation,
    // and returns an error if the agent process exits non-zero or fails.
    RunSession(ctx context.Context, runID string, prompt string) error
}
```

#### Factory & Future Extensibility
Agent instances are constructed via a centralized factory (`agent.NewRunner`), which maps the configured agent type to the corresponding `AgentRunner` implementation:

```go
type Config struct {
    Type   string                 `mapstructure:"type"`   // e.g., "codex" (default in V0.1.0)
    Codex  CodexConfig            `mapstructure:"codex"`  // Codex-specific configuration
    Env    map[string]string      `mapstructure:"env"`    // Environment variables
}

func NewRunner(cfg *Config) (AgentRunner, error) {
    switch cfg.Type {
    case "codex", "": // default to codex in V0.1.0
        return NewCodexRunner(&cfg.Codex, cfg.Env), nil
    default:
        return nil, fmt.Errorf("unsupported agent type %q: V0.1.0 only supports 'codex'", cfg.Type)
    }
}
```

**Adding Future Agents:**  
When support for a new agent (e.g., `Claude`, `Pi`) is introduced in subsequent releases:
1. Implement the `AgentRunner` interface in a dedicated file (e.g., `internal/agent/claude.go`).
2. Add the agent-specific configuration struct under `agent.<name>` in `ralph.json`.
3. Add a new case to `NewRunner` in `internal/agent/factory.go`.
No changes are required in the orchestrator, workspace, prompt builder, or pipeline control flows.

### 4.2 Codex CLI Implementation (`CodexRunner`)

In V0.1.0, the `CodexRunner` (`internal/agent/codex.go`) is the concrete implementation of `AgentRunner`. It invokes the Codex CLI non-interactively using the `exec` subcommand:

```text
codex exec [flags] "<prompt>"
```

#### Invocation Details & Flag Mapping
- **Autonomous Execution:** Uses `codex exec` which runs autonomously without interactive confirmation prompts.
- **Working Directory:** Set to the repository root using `-C <repo_root>` (and the process working directory).
- **Sandboxing:** Passed via `-s <sandbox_mode>` (default: `workspace-write`), granting write access to the workspace while protecting the rest of the host system.
- **Model Selection:** If `agent.codex.model` is configured, passed via `-m <model>` (e.g., `-m o3`). If empty, Codex CLI's default model is used.
- **Reasoning Effort:** If `agent.codex.reasoning_effort` is configured, passed via config override flag `-c model_reasoning_effort="<effort>"` (e.g., `low`, `medium`, `high`).
- **Prompt Passing:** The generated markdown prompt is passed as the final positional argument to `codex exec`.
- **Process Lifecycle & Cancellation:** Spawns via `exec.CommandContext(ctx, ...)`. When an interrupt (`SIGINT`/`SIGTERM`) cancels the context, the Codex process and its children are cleanly terminated.
- **Structured Logging:** Full stdout and stderr from the Codex run are piped and persisted to `.ralph/runs/<run_id>/agent.log` for debugging and post-mortems.

### 4.3 Configuration (`ralph.json`)

System configuration resides in `ralph.json` at the repository root. In V0.1.0, the agent block is configured for Codex:

```json
{
  "agent": {
    "type": "codex",
    "codex": {
      "command": "codex",
      "model": "",
      "reasoning_effort": "",
      "sandbox": "workspace-write",
      "extra_args": []
    },
    "env": {}
  },
  "max_retries": 3
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `agent.type` | no | `"codex"` | The agent backend to use. In V0.1.0, only `"codex"` is valid (defaults to `"codex"` if omitted). Prepared for future extensions (e.g., `"claude"`, `"pi"`). |
| `agent.codex.command` | no | `"codex"` | The Codex CLI binary name or absolute path. Must be on `PATH`. |
| `agent.codex.model` | no | `""` | Model override passed via `-m <model>`. Empty uses Codex CLI defaults. |
| `agent.codex.reasoning_effort` | no | `""` | Reasoning effort override passed via `-c model_reasoning_effort="<effort>"`. |
| `agent.codex.sandbox` | no | `"workspace-write"` | Sandbox policy passed via `-s <sandbox_mode>` (`read-only`, `workspace-write`, `danger-full-access`). |
| `agent.codex.extra_args` | no | `[]` | Additional flags to append directly to `codex exec`. |
| `agent.env` | no | `{}` | Additional environment variables injected into the agent process (merged with host environment). |
| `max_retries` | no | `3` | Max CI retry attempts (Phase C). |

### 4.4 Agent Contract

Regardless of the underlying agent backend (Codex in V0.1.0, or future runners), any agent invoked by Ralph must adhere to the standard Ralph agent contract:

1. **Run autonomously:** Execute non-interactively without user confirmation prompts.
2. **Workspace instruction awareness:** Automatically read and adhere to repo-level instructions from `AGENTS.md` (or tool-specific formats like `CLAUDE.md` provided in the repository). The host will not embed these files into the prompt.
3. **Completion Signal:** Write `ralph.done` at repo root when the task is complete (content: `success`).
4. **Escalation Signal:** Write `ralph.escalate` at repo root if blocked (content: one-paragraph blocker description).
5. **Exit Status:** Exit code 0 on success, non-zero on unrecoverable error (host treats non-zero exit as escalation).

The host does **not** parse agent stdout/stderr. File IPC signals are the only contract.

---

## 5. Skill Contracts

In the Ralph system, "skills" are behavioral contracts written as natural-language instructions in `AGENTS.md` (or tool-specific files such as `CLAUDE.md` provided in the repository). We assume the operator uses modern coding agents that automatically ingest repo-level instruction files; therefore, the host does not embed `AGENTS.md` into prompts. The agent reads these repository files directly to know *how* to execute tasks in the repository.

For V0.1.0, the following skill contracts must be defined in `AGENTS.md` and adhered to by the agent across its isolated sessions.

### 5.1 Implementation Phase Rules (Commit & Handoff)
The instructions must mandate the following behaviors during Phase A:
- **Logical Chunking:** Run the full test suite after completing a logical chunk of work.
- **Commit Format:** Use Conventional Commits (`<type>(<scope>): <description>`). Append `(#<issue_number>)` to the description.
- **Handoff Documentation:** Before writing `ralph.done`, you MUST append a brief, bulleted summary of what you implemented, new files created, or major architectural decisions to `.ralph-handoff.md`. This is critical because the next agent session will rely on this file to understand your work.

### 5.2 Draft-PR Skill (Unified Mode)
The instructions must mandate the following behaviors during Phase B:
- **Scope:** Your only task in this session is to write the PR description. Do not write implementation code.
- **Output:** Write a markdown file to `.ralph/draft-pr.md`.
- **Content Requirements:**
  - **Title line:** A high-level title summarizing the unified scope.
  - **Linked Issues:** A section explicitly stating `Closes #<issue_number>` for every issue provided.
  - **The "How":** A narrative plain-English section explaining what was changed and why across the entire batch, using the contents of `.ralph-handoff.md` as your primary reference.
- **Pushing:** After writing the file, push the branch to the remote (`git push -u origin HEAD`), then write `ralph.done`.

### 5.3 CI Fix Skill
The instructions must mandate the following behaviors during Phase C:
- **Scope:** You are responding to a CI failure. Read the provided logs, fix the offending code/tests, ensure local tests pass, and amend or create a new commit.
- **Pushing:** Push the branch (using force-push if you amended), then write `ralph.done`.

---

## 6. Implementation Phasing

To build this vertical slice efficiently, the implementation should be ordered as follows:

### Phase 1: Orchestration & Handoff Infrastructure
- Implement CLI arguments parsing for an arbitrary list of issue numbers.
- Create the outer loop that iterates through the provided issues.
- Implement `.ralph-handoff.md` creation and state management between loops.
- Implement the `AgentRunner` interface and the initial `CodexRunner` adapter executing `codex exec`.
- Implement the agent factory (`agent.NewRunner`) with support for the `"codex"` type and validation for future agent backends.
- *Test:* Run the script with dummy issues and ensure the host correctly queries GitHub, isolates specs, generates sequential prompts, and waits for `ralph.done` in each step.

### Phase 2: Dedicated PR Session
- Write the Draft-PR skill instructions into `AGENTS.md` (and ensure appropriate agent config files/symlinks exist in the repo).
- Implement the host logic to spawn the dedicated PR-drafting session once Phase 1 is complete.
- Implement reading `.ralph/draft-pr.md` and calling `gh pr create`.
- *Test:* Run an end-to-end flow where multiple issues are implemented, their summaries are handed off, and a cohesive PR is opened on GitHub.

### Phase 3: The CI Loop
- Implement `gh pr checks` polling.
- Implement failure detection and log extraction.
- Implement the retry session (spawning a fresh agent window specifically for fixing the logs).
- *Test:* Intentionally introduce a test failure, verify the host catches it, spawns the CI-fix agent, and successfully recovers.

---

## 7. Architecture & Technical Decisions

The following technical foundations govern the V0.1.0 Go implementation:

### 7.1 Core Tech Stack
The CLI orchestrator is built in **Go**, utilizing **Cobra** for command routing and **Viper** for configuration management (`ralph.json`).

### 7.2 Git & GitHub Integration
Instead of using native API libraries, the system **shells out to the `gh` and `git` CLIs** via `os/exec`. This leverages the user's existing authentication state and simplifies JSON extraction (e.g., `gh issue view --json`).

### 7.3 Process Lifecycle & Context Cancellation
To prevent orphaned agent processes from modifying the repository in the background, process lifecycles are strictly managed. The application uses `signal.NotifyContext(context.Background(), os.Interrupt)` to wire interrupt signals (Ctrl+C) to a global `context.Context`. All agent subprocesses are spawned using `exec.CommandContext`, ensuring that a cancellation automatically terminates the child process and safely exits the orchestrator.

### 7.4 File IPC & Signal Monitoring
Communication from the agent to the host (via `ralph.done` or `ralph.escalate`) is detected using **interval polling** (a 5-second `time.Ticker`), rather than filesystem watchers, to ensure robust cross-platform compatibility.

### 7.5 Application Boundaries & Testability
To allow for fast unit testing of the complex Phase A/B/C orchestration loops without triggering real AI agents or network calls, external side-effects are abstracted behind distinct interfaces:
- `GitHubClient`: Defines methods like `GetIssue`, `CreatePR`, `GetCIStatus`, and `GetFailedLogs`.
- `GitClient`: Defines methods like `CreateBranch` and `Push`.
- `AgentRunner`: Defines `RunSession(ctx context.Context, runID string, prompt string) error`. Decouples orchestration from the concrete agent CLI (`CodexRunner` in V0.1.0), facilitating straightforward addition of future agent backends.

### 7.6 Error Taxonomy & Control Flow
The orchestrator uses typed Go errors to cleanly separate internal failures from expected operational escalations:
- `ErrSystem`: Missing configurations, bad CLI flags, missing binaries (Standard Exit 1).
- `ErrEscalation`: The agent explicitly signaled it was blocked via `ralph.escalate` (Graceful Exit 1 with logged context).
- `ErrCIFailure`: The system exhausted all `max_retries` during Phase C (Graceful Exit 1 with failure output).

### 7.7 Prompt Assembly Engine
Context and prompt assembly uses Go's **`text/template`** engine. Templates are maintained as distinct `.tmpl` files bundled into the binary via `//go:embed`. This decouples the prompt phrasing and layout from the Go orchestration code. Prompts assemble issue bodies, referenced specs, and handoff notes without inlining `AGENTS.md`, relying on the agent's native ability to read repository instructions directly.

### 7.8 CLI UX and Logging
- **Agent Output Logging:** The host automatically captures the full stdout and stderr for every agent session (issue runs, PR drafting, CI fixes) and saves it to session-specific log files (e.g., `.ralph/runs/<run_id>/`). This provides structured output for post-mortem analysis.
- **Terminal UI:** The CLI simply prints what issue number or phase is currently being worked on (e.g., "Working on Issue #42..."). Spinners are not needed. Agent logs are not printed to stdout.