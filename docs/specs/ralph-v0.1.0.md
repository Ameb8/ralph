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

### 1.2 Out of Scope for V0.1.0
- **Queue Management:** No polling for `ralph-ready` labels.
- **Dependency Resolution:** The `Depends On` issue relationships are ignored; the user manually ensures the provided issues are ready.
- **Scope Enforcement (Diff checking):** Bypassed for V0.1.0.
- **GitHub Action PR Trigger:** Instead of a separate `.github/workflows/draft-pr.yml` workflow, the V0.1.0 host script will directly execute `gh pr create` to simplify the initial infrastructure footprint.
- **Verbose Output & Live Logs:** A `--verbose` flag or streaming live agent stdout/stderr to the user's terminal is out of scope.

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
     - `AGENTS.md`
     - The current `.ralph-handoff.md` (what previous sessions did)
     - The specific Issue body and its Specs
   - Host spawns a **fresh agent session**.
   - Agent implements the issue, commits chunks, and appends a summary of its work to `.ralph-handoff.md`.
   - Agent signals completion via `ralph.done`. Host terminates this session and moves to the next issue.
   - *Escalation Exception:* If `ralph.escalate` is detected, the host prints the blocker and exits 1 immediately.

### Phase B: PR Drafting
1. **Context Assembly:** After all issues are completed, the host builds a new prompt containing:
   - All target Issue bodies.
   - The final `.ralph-handoff.md` (the summary of all implemented work).
   - `AGENTS.md` (specifically for the Draft-PR skill).
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

V0.1.0 supports a configurable agent invocation mechanism so the host can work with any agent that accepts a single-shot prompt (pi, codex, claude, etc.).

### 4.1 Configuration (`ralph.json`)

```json
{
  "agent": {
    "command": "pi",
    "args": ["run", "--prompt"],
    "prompt_delivery": "arg",
    "env": {}
  },
  "max_retries": 3
}
```

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `command` | **yes** | — | The command to invoke (e.g., `pi`, `codex`, `claude`). Must be on PATH or absolute path. |
| `args` | **yes** | — | Array of base arguments. The prompt is appended or injected based on `prompt_delivery`. |
| `prompt_delivery` | **yes** | — | One of: `arg` (appends prompt as final arg), `stdin` (writes prompt to stdin), `file` (writes prompt to a temp file, passes path as final arg). |
| `env` | no | `{}` | Additional environment variables (e.g., API keys). Merged with process env. |
| `max_retries` | no | `3` | Max CI retry attempts (Phase C). |

### 4.2 Prompt Delivery Modes

| Mode | Invocation Example |
|------|-------------------|
| `arg` | `pi run --prompt "$PROMPT"` |
| `stdin` | `pi run --prompt-stdin` (writes prompt to stdin) |
| `file` | `pi run --prompt-file /tmp/ralph-prompt-123.md` (writes temp file, passes path) |

The host writes the prompt to a temp file for `file` mode, invokes the command, then deletes the temp file after the agent exits.

### 4.3 Agent Contract

Any agent invoked by the host must:

1. **Accept the prompt** via the configured delivery mode (`arg`, `stdin`, or `file`).
2. **Run autonomously** — no interactive prompts, no confirmation prompts.
3. **Write `ralph.done`** at repo root when the task is complete (content: `success`).
4. **Write `ralph.escalate`** at repo root if blocked (content: one-paragraph blocker description).
5. **Exit 0** on success, non-zero on error (host treats non-zero as escalation).

The host does **not** parse agent stdout/stderr. Signal files are the only contract.

### 4.4 Example Configurations

**pi (stdin prompt delivery):**
```json
{
  "agent": {
    "command": "pi",
    "args": ["run", "--prompt-stdin"],
    "prompt_delivery": "stdin"
  }
}
```

**codex (arg delivery):**
```json
{
  "agent": {
    "command": "codex",
    "args": ["exec"],
    "prompt_delivery": "arg"
  }
}
```

**claude (file delivery):**
```json
{
  "agent": {
    "command": "claude",
    "args": ["-p"],
    "prompt_delivery": "file"
  }
}
```

---

## 5. Skill Contracts

In the Ralph system, "skills" are behavioral contracts written as natural-language instructions in `AGENTS.md`. The agent reads these to know *how* to execute tasks in the repository. 

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
- *Test:* Run the script with dummy issues and ensure the host correctly queries GitHub, isolates specs, generates sequential prompts, and waits for `ralph.done` in each step.

### Phase 2: Dedicated PR Session
- Write the Draft-PR skill instructions into `AGENTS.md`.
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
- `AgentRunner`: Defines `RunSession(ctx, prompt)`.

### 7.6 Error Taxonomy & Control Flow
The orchestrator uses typed Go errors to cleanly separate internal failures from expected operational escalations:
- `ErrSystem`: Missing configurations, bad CLI flags, missing binaries (Standard Exit 1).
- `ErrEscalation`: The agent explicitly signaled it was blocked via `ralph.escalate` (Graceful Exit 1 with logged context).
- `ErrCIFailure`: The system exhausted all `max_retries` during Phase C (Graceful Exit 1 with failure output).

### 7.7 Prompt Assembly Engine
Context and prompt assembly uses Go's **`text/template`** engine. Templates are maintained as distinct `.tmpl` files bundled into the binary via `//go:embed`. This decouples the prompt phrasing and layout from the Go orchestration code.

### 7.8 CLI UX and Logging
- **Agent Output Logging:** The host automatically captures the full stdout and stderr for every agent session (issue runs, PR drafting, CI fixes) and saves it to session-specific log files (e.g., `.ralph/runs/<run_id>/`). This provides structured output for post-mortem analysis.
- **Terminal UI:** The CLI simply prints what issue number or phase is currently being worked on (e.g., "Working on Issue #42..."). Spinners are not needed. Agent logs are not printed to stdout.