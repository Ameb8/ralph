# Ralph

Ralph is an automated issue-driven development CLI tool.

## Build

Compile the project using standard Go build commands:

```bash
go build -o bin/ralph ./cmd/ralph
```

## Run

Run the unified PR pipeline by passing one or more issue numbers:

```bash
./bin/ralph run <issue_1> [<issue_2> ...]
```

**Example:**
```bash
./bin/ralph run 42 45
```

## Demo Script (`ralph.sh`)

The repository includes a bash demo script (`demo/ralph.sh`) that automates solving a list of GitHub issues one-by-one using the Codex CLI, following a solve-then-commit pipeline.

### Prerequisites
- `gh` (GitHub CLI), authenticated and on your `PATH`.
- `codex` CLI installed and on your `PATH`.
- `jq` installed and on your `PATH`.
- Bash 3.2 or later (including the Bash version bundled with macOS).

### Installation

To invoke the script from any directory, you can copy it to a directory on your system's `PATH`.

**For Linux and macOS (System-wide using sudo):**
```bash
chmod +x demo/ralph.sh
sudo cp demo/ralph.sh /usr/local/bin/ralph
```

**For Linux and macOS (User-local without sudo):**
```bash
chmod +x demo/ralph.sh
mkdir -p ~/.local/bin
cp demo/ralph.sh ~/.local/bin/ralph
```
*(If using the user-local method, ensure `~/.local/bin` is added to your `PATH` in your `~/.bashrc`, `~/.zshrc`, or equivalent config file).*

### Usage

The script requires a prompt template file and the issue numbers you want to solve.

**Running with issue numbers:**
```bash
ralph <prompt.md> <issue-number> [issue-number ...]
```

**Running with an issues file:**
```bash
ralph <prompt.md> --issues-file issues.txt
```

### Run Codex within the repository only

Run `ralph` from the root of the Git repository it should modify. By default,
the script invokes every Codex step with `-C "$(pwd -P)"` and
`-s workspace-write`. `workspace-write` permits writes in that repository
root while preventing Codex-initiated file changes outside it.

```bash
cd /path/to/repository
RALPH_CODEX_SANDBOX=workspace-write ralph prompt.md --issues-file issues.txt
```

Do not use `--dangerously-bypass-approvals-and-sandbox`, `danger-full-access`,
or the older informal term `--yolo` for this loop: those remove the repository
boundary. `RALPH_CODEX_SANDBOX=read-only` is available for inspection-only runs, but
the solve and commit steps will then be unable to modify or commit files.

**Prompt Template Guidelines:**
The `<prompt.md>` file serves as the base instructions for solving the issue. If you include the token `{{ISSUE}}` anywhere in the file, it will be replaced with the fetched GitHub issue title, body, and URL. If the `{{ISSUE}}` token is missing, the script will automatically append the issue contents to the end of the prompt.

### Environment Variables

You can configure the behavior of the script via the following environment variables:

- `REPO`: The target repository in `owner/repo` format (default: uses `gh`'s current directory detection).
- `MAX_RETRIES`: Retry count for the code modification/solve step (default: `1`).
- `COMMIT_MAX_RETRIES`: Retry count for the commit step (default: `1`).
- `SIGNAL_DIR`: Directory where `.done` and `.escalate` signals, as well as logs, are saved (default: `.ralph`).
- `SOLVE_MODEL`: The `codex --model` to use for the issue-solving step.
- `SOLVE_REASONING_EFFORT`: Reasoning effort for the issue-solving step. The script passes this as `-c model_reasoning_effort=<value>`.
- `COMMIT_MODEL`: The `codex --model` to use for the commit step.
- `COMMIT_REASONING_EFFORT`: Reasoning effort for the commit step, passed using the same configuration override.
- `RALPH_CODEX_SANDBOX`: Codex sandbox mode for both steps (default: `workspace-write`). Keep this at `workspace-write` to prevent Codex from modifying files outside the repository root where you started `ralph`. The legacy `CODEX_SANDBOX` name is accepted only for `read-only` and `workspace-write` values.

**Configuration Example:**
You can specify different models and reasoning efforts for solving versus committing. For example, using a stronger model for solving and a faster/cheaper model for creating the commit:

```bash
SOLVE_MODEL=o3 \
SOLVE_REASONING_EFFORT=high \
COMMIT_MODEL=o4-mini \
COMMIT_REASONING_EFFORT=low \
ralph prompt.md 101 102 103
```
