# Ralph

Ralph is an autonomous, issue-driven development CLI tool. Given a list of GitHub issues, Ralph orchestrates an AI coding agent through sequential, fresh-context sessions to implement code changes, open a unified Pull Request, and monitor and resolve any continuous integration (CI) failures.

### Key Highlights

- **Sequential Fresh Contexts:** Prevents LLM context saturation and prompt degradation by running an isolated agent session for each issue, a dedicated session for drafting the PR, and separate retry sessions for CI fixes.
- **Inter-Session Handoff:** Tracks architectural notes, changes, and decisions across agent sessions via a lightweight `.ralph-handoff.md` file.
- **Unified Pull Requests:** Implements related issues together on a single branch (`ralph/unified-<issues>`) and automatically creates a cohesive GitHub PR describing all changes.
- **Automated CI Recovery:** Watches GitHub Actions check runs; if checks fail, Ralph extracts failure logs and dispatches an agent session to diagnose and push a fix.

---

## Prerequisites

Before using Ralph, ensure the following tools are installed and available on your `PATH`:

- **Go 1.26+** (for building or running from source)
- **Git CLI (`git`)**: Installed and configured.
- **GitHub CLI (`gh`)**: Installed and authenticated (`gh auth status`). Ralph uses `gh` to read issues, open PRs, and monitor CI checks.
- **Agent CLI**: [OpenAI Codex CLI](https://github.com/openai/codex) (`codex`) installed and authenticated on your machine.

---

## Installation & Building

### Option 1: Install with `go install`

Install the `ralph` binary directly into your `$GOPATH/bin` (or `$HOME/go/bin`):

```bash
go install ./cmd/ralph
```

Ensure `$GOPATH/bin` (or `~/go/bin`) is included in your shell's `PATH`.

### Option 2: Build from Source

Clone the repository and compile the binary:

```bash
# Using Go directly
go build -o bin/ralph ./cmd/ralph

# Or using the Makefile
make build
```

This generates the binary at `bin/ralph`. You can optionally move it to a system-wide directory:

```bash
sudo cp bin/ralph /usr/local/bin/ralph
```

---

## Configuration (`ralph.json`)

Ralph is configured via a `ralph.json` file located at the repository root, or specified with the `--config` flag.

### Example `ralph.json`

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

### Configuration Settings

| Setting | Type | Default | Description |
|---|---|---|---|
| `agent.type` | string | `"codex"` | AI agent backend to use (`codex`). |
| `agent.codex.command` | string | `"codex"` | Path or command name for the Codex CLI binary. |
| `agent.codex.model` | string | `""` | Optional model override (passed via `-m <model>`, e.g. `o3`). Uses Codex CLI defaults if empty. |
| `agent.codex.reasoning_effort` | string | `""` | Optional reasoning effort (passed via `-c model_reasoning_effort="<effort>"`: e.g. `low`, `medium`, `high`). |
| `agent.codex.sandbox` | string | `"workspace-write"` | Sandbox mode passed via `-s <sandbox_mode>` (`workspace-write`, `read-only`, `danger-full-access`). |
| `agent.codex.extra_args` | string array | `[]` | Additional CLI flags appended directly to `codex exec`. |
| `agent.env` | object | `{}` | Additional environment variables injected into the agent process. |
| `max_retries` | integer | `3` | Maximum number of CI fix attempts in Phase C before exiting with a failure. |

---

## Repository Setup (`AGENTS.md`)

Ralph relies on repository-level instructions defined in `AGENTS.md` (or `CLAUDE.md`). Modern coding agents ingest repository instructions natively, keeping prompt tokens efficient.

Ensure your repository contains an `AGENTS.md` file at its root defining the agent skill contracts:

1. **Implementation Phase (Phase A):** Committing in logical chunks (`<type>(<scope>): <desc> (#<issue>)`), running test suites before concluding, appending notes to `.ralph-handoff.md`, and signaling completion via `ralph.done` (or `ralph.escalate` if blocked).
2. **Draft-PR Skill (Phase B):** Writing a unified PR description to `.ralph/draft-pr.md`, linking issues (`Closes #<issue>`), pushing the branch to the remote (`git push -u origin HEAD`), and signaling `ralph.done`.
3. **CI Fix Skill (Phase C):** Analyzing failure logs, diagnosing issues, verifying local test passes, committing and pushing fixes, and signaling `ralph.done`.

---

## Usage

### 1. Pre-flight Validation

Before running a batch of issues, verify your environment, configuration, and dependencies:

```bash
ralph validate
```

This performs automated pre-flight checks:
- Verifies `AGENTS.md` is present and non-empty at the repository root.
- Validates the syntax and values in `ralph.json`.
- Confirms the `git` CLI is available in `PATH`.
- Confirms the `gh` CLI is available in `PATH`.
- Confirms the configured agent CLI (`codex`) is available in `PATH`.
- Verifies no active `.ralph-lock` file exists from another running instance.

**Helpful flags:**
- `--quiet`: Print only failure messages.
- `--json`: Output check results in structured JSON format.

```bash
ralph validate --quiet
ralph validate --json
```

### 2. Running the Unified Issue Pipeline

Run Ralph by passing one or more GitHub issue numbers from within the target Git repository:

```bash
ralph run <issue_1> [<issue_2> ...]
```

**Examples:**

```bash
# Implement a single issue
ralph run 42

# Implement multiple issues in a single unified PR
ralph run 42 45 48

# Run with a custom configuration file
ralph run --config ./config/custom-ralph.json 101 102
```

### How the Pipeline Works

When `ralph run` is invoked, Ralph guides the task through three distinct phases:

```text
[Operator] ralph run 42 45
    │
    ▼
[Workspace Setup] Acquires .ralph-lock, creates branch ralph/unified-42-45
    │
    ├─► Phase A: Sequential Issue Implementation
    │     ├── Issue #42: Fresh agent context -> implements code -> commits -> updates .ralph-handoff.md -> ralph.done
    │     └── Issue #45: Fresh agent context -> implements code -> commits -> updates .ralph-handoff.md -> ralph.done
    │
    ├─► Phase B: Unified PR Drafting
    │     └── Fresh agent context reads all issues & .ralph-handoff.md
    │         -> writes .ralph/draft-pr.md -> pushes branch -> gh pr create opens PR
    │
    └─► Phase C: CI Polling & Auto-Fix Retry Loop
          ├── Polls GitHub Actions checks
          ├── Success: All checks pass -> exits 0
          └── Failure: Extracts failure logs -> spawns fresh agent context to fix -> retries (up to max_retries)
```

1. **Workspace Safety Lock & Branching:** Ralph creates `.ralph-lock` to prevent conflicting concurrent executions, initializes `.ralph-handoff.md`, and creates a dedicated branch named `ralph/unified-<issues>`.
2. **Phase A (Sequential Issue Implementation):** For each issue provided:
   - Fetches the issue title, body, and any referenced specification files (`docs/specs/*.md`) using GitHub CLI.
   - Spawns a fresh agent session (`codex exec`) with issue details, referenced specs, and prior handoff context.
   - The agent writes the implementation, verifies changes against tests, creates commits, appends an architectural summary to `.ralph-handoff.md`, and signals completion via `ralph.done`.
3. **Phase B (Pull Request Drafting):**
   - Spawns an isolated agent session dedicated solely to writing the PR description.
   - The agent analyzes `.ralph-handoff.md` and the original issue requirements, generates `.ralph/draft-pr.md`, and pushes the branch to the remote repository.
   - Ralph reads `.ralph/draft-pr.md` and opens the pull request using `gh pr create`.
4. **Phase C (CI Polling & Retries):**
   - Monitors CI check status for the branch via GitHub Actions.
   - If CI checks pass, Ralph exits cleanly with code 0.
   - If CI checks fail, Ralph retrieves failure logs (`gh run view --log-failed`), spawns a fresh agent session with the logs and handoff context, and instructs the agent to diagnose, fix, commit, and push.
   - Retries up to `max_retries` (default: 3).

### Logs & Troubleshooting

- **Agent Logs:** All output (stdout and stderr) from every agent session is saved to `.ralph/runs/<run_id>/agent.log` (e.g. `.ralph/runs/issue-42/agent.log`, `.ralph/runs/draft-pr/agent.log`, `.ralph/runs/ci-fix-1/agent.log`).
- **Escalation (`ralph.escalate`):** If an agent cannot proceed (e.g. due to ambiguous specifications or conflicting requirements), it writes a blocker explanation to `ralph.escalate`. Ralph will cleanly abort and display the escalation reason.
- **Lockfile (`.ralph-lock`):** If a previous execution terminated abruptly, Ralph checks whether the recorded process is still alive. Stale lockfiles are automatically cleared; active locks from concurrent processes will cause Ralph to halt safely.

---

## Development

Ralph includes a `Makefile` for common development tasks:

```bash
# Build the binary
make build

# Run unit tests
make test

# Run unit tests with race detection
make test-race

# Run unit tests with coverage profile
make test-cover

# Check code formatting and run go vet
make lint

# Run pre-flight checks on local workspace
make validate

# Clean build artifacts
make clean
```
