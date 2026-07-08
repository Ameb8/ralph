# Ralph — Automated Issue-Driven Development System
## Behavioral Specification

---

## 1. Purpose

Ralph is an automated development system that takes a queue of well-specified GitHub issues and produces reviewed-ready pull requests without human involvement between those two points. A human authors specs and creates issues. Ralph implements them. A human reviews and merges the resulting PRs.

The system is not an AI coding assistant. It is a closed-loop pipeline in which an AI coding agent is one component, orchestrated by a host process and bounded by contracts defined in repository configuration files.

---

## 2. System Components

The system is composed of five distinct components. Each has a defined responsibility and defined boundaries.

### 2.1 Host Process

The host process is the orchestrator. It is a CLI tool (`ralph`) that a human or cron job invokes. It is responsible for:

- Selecting the next eligible issue from GitHub
- Assembling all context the agent will need (issue body, spec files, repo conventions)
- Spawning the agent with a single, complete initial prompt
- Monitoring the agent's completion signal
- Monitoring CI status on the resulting PR
- Driving the retry loop if CI fails
- Escalating to GitHub if the agent or CI cannot recover
- Notifying the operator on success

The host process never implements code. It has no opinion about what the agent produces. Its job is orchestration, context assembly, and lifecycle management.

### 2.2 Agent

The agent is an AI coding tool that operates autonomously inside the repository. It receives a single initial prompt assembled by the host and works until it either completes the task or determines it is blocked.

The agent is responsible for:

- Reading and understanding the task from its initial prompt
- Implementing the acceptance criteria
- Running the test suite, linter, and formatter after each logical chunk of work
- Invoking the commit skill to commit completed chunks
- Invoking the draft-PR skill to produce the PR body
- Pushing the branch
- Writing a completion signal file when done or blocked

The agent is not responsible for selecting its own issue, reading its own config, or understanding the loop infrastructure. That context is pre-assembled and handed to it.

### 2.3 Repository Configuration Files

Two files in the repository define the contracts the agent must follow. These are the only repo-level files the agent reads as behavioral instructions.

**`AGENTS.md`** — repo-specific conventions. Contains:
- Exact commands to run the test suite
- Exact commands to run the linter and formatter, in order
- Project structure: where packages live, naming conventions
- The commit skill: how to write commit messages, how to stage files
- The draft-PR skill: how to generate the PR body's "How" section
- Hard constraints: files or directories the agent must never modify

**`AGENTS.md` is the single source of truth for how work is done in a given repo.** It is maintained by the human operator and updated as the codebase evolves. The agent treats it as authoritative.

`AGENTS.md` is not loop-specific. It describes how any developer (human or agent) should work in the repo.

### 2.4 GitHub Actions Workflow

A workflow file in `.github/workflows/` triggers on pushes to `issue/**` branches. Its sole responsibility is opening a draft PR when one does not yet exist for that branch. It:

- Extracts the issue number from the branch name
- Fetches the issue body from the GitHub API
- Opens a draft PR using a template, linking it to the issue via `Closes #N`
- Does nothing else — it does not run tests, it does not merge, it does not notify

CI (tests, lint) runs on the same PR through whatever workflow the repo already has. The draft-PR workflow is separate from CI.

### 2.5 Issue Queue

The issue queue is a set of GitHub issues that carry the `ralph-ready` label and are in the open state with no assignee. Issues in the queue must follow a defined template (see Section 4). The human operator manages the queue by authoring issues, applying or removing labels, and writing or updating the referenced spec documents.

---

## 3. System Lifecycle

### 3.1 Steady State

In steady state, the system operates as follows:

```
operator creates issue → applies ralph-ready label
       ↓
host selects issue → assembles context → spawns agent
       ↓
agent implements → commits → pushes branch
       ↓
GitHub Actions opens draft PR
       ↓
CI runs on draft PR
       ↓
host monitors CI → notifies operator on green
       ↓
operator reviews PR → merges → issue auto-closes
```

This cycle repeats for each issue in the queue. The host may process issues sequentially or, if designed to do so, in parallel on separate branches.

### 3.2 Issue Selection

The host selects the next issue by querying GitHub for open issues that are:

- Labeled `ralph-ready`
- Unassigned
- Eligible given dependency state (see Section 5.2)

Among eligible issues, priority is:

1. Issues labeled `priority: high`
2. Issues belonging to the active milestone
3. Lowest issue number (FIFO)

The host self-assigns the selected issue immediately upon selection to prevent a concurrent host process from picking the same issue.

### 3.3 Context Assembly

Before spawning the agent, the host assembles all context the agent will need. The agent receives this as its initial prompt and does not need to query GitHub or read config files independently.

Assembled context includes:

- Issue number, title, and body (summary, acceptance criteria, notes)
- Full contents of every spec file listed in the issue's `Specs` section
- Full contents of `AGENTS.md`
- A short rules block (see Section 3.4)
- Current branch name (already created by the host)
- Confirmation that the branch is checked out and based on a fresh pull of main

### 3.4 Agent Rules Block

The host injects a short, fixed rules block into every initial prompt. This is the only loop-specific instruction the agent receives:

- Work in logical chunks. Run the full test suite after each chunk before proceeding to the next.
- Do not ask for human input at any point.
- If you are genuinely blocked and cannot proceed, write the file `ralph.escalate` containing one paragraph describing the blocker, and stop.
- When all acceptance criteria are met, the suite is green, commits are made, and the branch is pushed, write the file `ralph.done` containing `success`.
- Do not modify files outside the scope implied by the issue and its referenced specs.

The agent is not told it is in a "ralph loop." It is told it has a task, here is the context, here are the rules, implement it.

### 3.5 Completion Detection

The host monitors the repository for one of two signal files written by the agent:

- **`ralph.done`** — agent considers the task complete. Host proceeds to CI monitoring.
- **`ralph.escalate`** — agent is blocked. Host reads the file, posts its contents as a GitHub issue comment, removes the `ralph-ready` label, adds `needs-human`, and notifies the operator.

The host does not parse agent output or terminal text. Signal files are the only IPC mechanism between host and agent.

### 3.6 CI Monitoring and Retry

After the agent writes `ralph.done` and the branch is pushed:

1. The GitHub Actions workflow opens a draft PR
2. CI runs on the PR
3. The host polls CI status

If CI is green, the host sends a Chime (or equivalent) notification and exits successfully.

If CI fails, the host enters a retry loop:

- It reads the CI failure output
- It sends a continuation prompt to the agent (appended to the same session, preserving context) containing the failure logs and an instruction to fix and force-push
- It waits for a new `ralph.done` signal
- It re-checks CI

This repeats up to `max_retries` times (default: 3). If CI is still failing after all retries, the host escalates identically to an agent-blocked escalation, adding the CI failure log to the GitHub comment.

Retries use continuation prompts, not fresh agent sessions. This preserves the agent's working context from the initial run.

---

## 4. Issue Template Contract

Every issue the system processes must conform to this structure. Sections are parsed by the host; deviations cause the issue to be skipped.

```markdown
## Summary
One paragraph describing what needs to be implemented.

## Specs
- `docs/specs/filename.md`
- `docs/specs/other.md`

## Depends On
- #38
- #41

## Acceptance Criteria
- [ ] All existing tests pass
- [ ] New behavior is covered by tests
- [ ] Linter and formatter clean
- [ ] No files modified outside expected scope
- [ ] <issue-specific criteria>

## Notes
Optional: edge cases, known constraints, implementation hints.
```

**`Specs`** — paths to canonical spec documents the agent must read in full before implementing. These are relative to the repo root. Every spec path listed must exist on disk at the time the host assembles context; a missing spec causes the host to escalate rather than proceed with incomplete context.

**`Depends On`** — issue numbers that must be closed before this issue is eligible for selection. Omit the section entirely if there are no dependencies.

**`Acceptance Criteria`** — the agent's exit condition. The agent does not mark these checkboxes; they are the human reviewer's checklist on the resulting PR. The agent uses them as its definition of done.

---

## 5. Dependency Resolution

### 5.1 Format

Dependencies are expressed as issue numbers in the `Depends On` section of the issue body. The host parses this section and resolves each referenced issue's state before considering the issue eligible.

### 5.2 Eligibility Rule

An issue is eligible for selection if and only if every issue listed in its `Depends On` section is in the closed state on GitHub. An issue with no `Depends On` section is unconditionally eligible (subject to label and assignee filters).

### 5.3 Sequencing

Dependency resolution allows the operator to queue an entire milestone of issues at once, including issues that depend on work not yet started. The host self-sequences them: as Ralph closes PRs (via merge), downstream issues become eligible and enter the queue automatically.

### 5.4 Circular Dependencies

The host detects circular dependency chains at startup and refuses to run, reporting which issues form the cycle. `ralph validate` also detects this as part of its pre-flight check.

---

## 6. Skill Contracts

Skills are behavioral contracts written as instructions in `AGENTS.md`. They are not code invoked by the host — they are instructions the agent reads and follows. The word "invoke" means the agent executes the steps described in the skill, not that a function is called.

### 6.1 Commit Skill

Triggered by the agent after each logical implementation chunk passes the test suite.

Behavior:
- Inspect the current diff to determine what was changed
- Determine commit type (`feat`, `fix`, `refactor`, `test`, `chore`, `docs`) from the nature of the change
- Determine scope from the primary package or service affected
- Write a commit message in the form `<type>(<scope>): <description> (#<issue_number>)`
- Stage only files relevant to this chunk — never stage everything
- Commit with the composed message

The commit skill never prompts for input and never produces a document for review. It commits directly.

### 6.2 Draft-PR Skill

Triggered by the agent after all chunks are committed and the suite is green, before pushing.

Behavior:
- Read the issue body for task context
- Read the list of commits on the branch
- Generate a "How" section: a plain-English narrative of what was changed and why, organized by commit
- Produce a complete PR body string using the repo's PR template, with the How section filled in

The draft-PR skill does not open the PR. The GitHub Actions workflow opens it. The skill produces the body text; the agent may use `gh pr edit` after the action creates the draft to set the body, or the body may be passed through the push trigger depending on implementation.

---

## 7. Escalation

The system escalates when it cannot make forward progress without human input. Escalation is not failure — it is the system correctly identifying that a human decision is required.

### 7.1 Escalation Triggers

| Trigger | Condition |
|---|---|
| Agent blocked | Agent writes `ralph.escalate` |
| CI unrecoverable | CI still failing after `max_retries` retry attempts |
| Missing spec | A file listed in the issue's `Specs` section does not exist |
| Scope violation | Agent modified files outside the expected scope of the issue |
| Circular dependency | Detected at startup or validation time |
| Merge conflict | Branch cannot be rebased onto main cleanly |
| Stale lockfile | A lockfile exists from a previous run whose process is no longer running |

### 7.2 Escalation Actions

When escalating, the host always:

1. Posts a comment on the issue explaining the escalation reason, last action attempted, and suggested next step
2. Removes the `ralph-ready` label
3. Adds the `needs-human` label
4. Notifies the operator via Chime (or equivalent)
5. Removes the lockfile
6. Exits non-zero

The human operator resolves the issue — updates a spec, fixes a merge conflict, clarifies an ambiguity — and re-applies `ralph-ready` when the issue is ready for another attempt.

---

## 8. Scope Enforcement

After the agent writes `ralph.done`, and before the host considers the run successful, it inspects the diff between the branch and main.

It compares the set of modified files against the expected scope — files plausibly related to the specs referenced in the issue and the conventions in `AGENTS.md`. The scope check is a smell test, not a whitelist: it flags unexpected modifications (e.g. changes to billing code on an auth issue) rather than enforcing a precise file list.

If unexpected files are modified, the host escalates with a scope violation rather than pushing. The operator reviews whether the modification is intentional (in which case the spec or issue may need updating) or a sign of spec ambiguity or agent drift.

---

## 9. Operator Touch Points

In normal operation, a human interacts with the system at exactly three points:

**1. Spec authorship**
Write or update canonical spec documents in `docs/specs/`. These are the ground truth the agent implements against. Spec quality is the primary determinant of output quality.

**2. Issue creation**
Convert specs into GitHub issues following the template in Section 4. Apply the `ralph-ready` label when an issue is ready to enter the queue. Manage `Depends On` relationships to sequence work correctly.

**3. PR review and merge**
Review the draft PR the system produces. Check that the acceptance criteria are met. Merge when satisfied. Merging auto-closes the issue via `Closes #N` in the PR body.

Everything between issue creation and PR review — branching, implementing, committing, opening the draft PR, running CI, retrying on failure, notifying — is handled by the system.

---

## 10. Operational Files

### 10.1 Lockfile

The host writes `.ralph-lock` at the repo root when it begins processing an issue. The file records the issue number, branch name, start time, and host process ID. The host removes it on clean exit (success or escalation).

A lockfile present when the host starts indicates either a concurrent run or a crashed previous run. The host checks whether the recorded process ID is still running. If yes, it exits without selecting an issue. If no, it logs a warning, removes the stale lockfile, and proceeds.

### 10.2 Run Log

The host maintains a structured log of each run, recording: which issue was selected, which spec files were loaded, when the agent was spawned, what the completion signal was, CI check results, and retry attempts. This log is the primary tool for post-mortem analysis when a run produces a low-quality PR or an unexpected escalation.

### 10.3 Signal Files

`ralph.done` and `ralph.escalate` are written by the agent at the repo root. The host deletes them at the start of each run to prevent stale signals from a previous run from being misread. The agent is instructed not to write either file until work is genuinely complete or genuinely blocked.

---

## 11. Validation

`ralph validate` is a pre-flight command that checks the system's readiness before a run. It is safe to run at any time and makes no changes.

Checks performed:

- `AGENTS.md` exists and contains non-stub content
- `ralph.json` (or equivalent config) is present and valid
- `gh` CLI is authenticated
- The GitHub Actions draft-PR workflow file is present
- At least one `ralph-ready` open issue exists
- All spec files referenced in `ralph-ready` issues exist on disk
- No circular dependencies exist among `ralph-ready` issues
- No stale lockfile is present
- Chime (or equivalent notification target) is reachable

Validation failures are reported with a description of the problem and the action required to resolve it.

---

## 12. Quality Model

The system's output quality is a function of three inputs, in order of impact:

**Spec quality** — vague or incomplete specs produce vague or incomplete implementations. The agent cannot infer intent that isn't written down. Escalations that cite "agent blocked" or "spec ambiguous" are signals that a spec needs updating, not that the agent failed.

**`AGENTS.md` quality** — if the agent doesn't know how the repo is structured, what tests to run, or how to commit, it will improvise. A well-maintained `AGENTS.md` is the difference between an agent that feels like a team member and one that feels like an outsider.

**Acceptance criteria specificity** — acceptance criteria are the agent's definition of done and the reviewer's checklist. Generic criteria ("tests pass") produce generic confidence. Specific criteria ("the `/auth/refresh` endpoint returns 401 on expired tokens") produce verifiable outcomes.

The CI pipeline and PR review step are the quality gate. The system is not designed to produce perfect output — it is designed to produce reviewable output that a human can accept or reject efficiently.


---
---

## 13. CLI Interface

The `ralph` binary is the operator's sole interface to the system. All commands operate on the current working directory, which must be the root of a repository configured for Ralph. Commands that interact with GitHub require the `gh` CLI to be authenticated.

Every command exits 0 on success and non-zero on any failure. Errors are written to stderr. Structured output (when requested via `--json`) is written to stdout. Human-readable output goes to stdout by default.

---

### 13.1 `ralph init`

Scaffolds the Ralph infrastructure files into the current repository. Intended to be run once when adopting Ralph in a new repo.

**Usage**
```
ralph init [flags]
```

**Behavior**

1. Checks that the current directory is a git repository. Refuses with an error if not.
2. Checks that `gh` is authenticated. Refuses if not.
3. Writes the following files, skipping any that already exist unless `--force` is given:
   - `.github/workflows/draft-pr.yml` — the GitHub Actions draft PR workflow
   - `.github/pr-template.md` — the PR body template
   - `ralph.json` — default configuration
   - `RALPH_SPEC.md` — this specification document, for operator reference (not included in agent context)
4. If `AGENTS.md` does not exist, writes a stub with clearly marked TODO sections and a warning that it must be completed before `ralph run` will succeed.
5. Creates `docs/specs/` if it does not exist.
6. Prints a summary of files written and a next-steps checklist:
   - Fill in `AGENTS.md` if it was stubbed
   - Write your first spec in `docs/specs/`
   - Create your first issue following the template in Section 4
   - Run `ralph validate` to confirm readiness

**Flags**

| Flag | Description |
|---|---|
| `--force` | Overwrite existing files. Without this flag, existing files are never modified. |
| `--skip-agents-stub` | Do not write `AGENTS.md` stub if `AGENTS.md` is absent. Use when you intend to write it manually. |

**Error conditions**

- Not a git repository → exit 1, explain
- `gh` not authenticated → exit 1, explain with `gh auth login` instruction
- File write failure → exit 1, report which file failed

---

### 13.2 `ralph validate`

Checks that the system is correctly configured and ready to run. Makes no changes to the repository or GitHub. Safe to run at any time.

**Usage**
```
ralph validate [flags]
```

**Behavior**

Runs all checks listed in Section 11. For each check, prints a pass/fail line. If any check fails, prints the failure reason and the action required to resolve it on the following line, indented.

Exits 0 only if all checks pass. Exits 1 if any check fails.

Example output (all passing):
```
✓ AGENTS.md present and non-stub
✓ ralph.json valid
✓ gh CLI authenticated
✓ GitHub Actions workflow present
✓ 3 ralph-ready issues found
✓ All spec files exist on disk
✓ No circular dependencies
✓ No stale lockfile
✓ Chime reachable
```

Example output (with failures):
```
✓ AGENTS.md present and non-stub
✓ ralph.json valid
✓ gh CLI authenticated
✓ GitHub Actions workflow present
✓ 2 ralph-ready issues found
✗ Spec file missing: docs/specs/billing.md
    Referenced by issue #47. Create the file or remove it from the issue's Specs section.
✓ No circular dependencies
✓ No stale lockfile
✗ Chime unreachable
    Check your Chime configuration in ralph.json and confirm the notification target is running.

2 checks failed.
```

**Flags**

| Flag | Description |
|---|---|
| `--json` | Output results as a JSON array of `{ check, passed, reason }` objects. |
| `--quiet` | Print only failures. Suppress passing checks. |

---

### 13.3 `ralph run`

Executes one or more loop iterations. By default runs once against the next eligible issue. Accepts an optional issue number to target a specific issue, and flags to run continuously or for a fixed number of iterations. This is the primary command.

**Usage**
```
ralph run [<issue>] [flags]
```

The optional `<issue>` argument is a GitHub issue number. When provided, Ralph runs against that specific issue instead of selecting from the queue. All other behavior is identical to a normal run.

**Single-iteration behavior (default)**

1. Checks for a stale lockfile. If found and the recorded process is still running, exits with an error. If stale (process dead), removes it and proceeds with a warning.
2. Cleans up any signal files (`ralph.done`, `ralph.escalate`) left from a previous run.
3. Runs a lightweight pre-flight check (subset of `ralph validate`): confirms `AGENTS.md` exists, `ralph.json` is valid, `gh` is authenticated. Does not run the full validation suite to avoid slowing the hot path. Full validation is the operator's responsibility before entering the queue.
4. If `<issue>` is given: validates that issue is open and `ralph-ready`, then uses it. If `<issue>` is not given: selects the next eligible issue per Section 3.2.
5. If no eligible issue is found (queue empty or all blocked on dependencies), prints a message and exits 2. This is not an error.
6. Assembles context per Section 3.3.
7. Writes the lockfile.
8. Creates the branch and checks it out.
9. Spawns the agent with the assembled initial prompt.
10. Polls for `ralph.done` or `ralph.escalate` at a configurable interval (default: 5 seconds).
11. On `ralph.done`: runs scope enforcement (Section 8), then proceeds to CI monitoring.
12. On `ralph.escalate`: executes escalation actions (Section 7.2) and exits 1.
13. On CI green: sends notification and exits 0.
14. On CI failure: enters retry loop (Section 3.6). After `max_retries` exhausted, escalates and exits 1.
15. Removes the lockfile on any clean exit (success or escalation).

**Multi-iteration behavior (`--loop` or `--count`)**

When `--loop` or `--count` is given, Ralph runs iterations sequentially — one issue at a time — rather than in parallel. Each iteration is a complete, independent run: the lockfile is acquired at the start of each iteration and released at the end before the next begins.

Between iterations, Ralph re-queries the issue queue. This means issues that became eligible mid-run (because a dependency was closed by a just-merged PR) are picked up correctly without restarting.

Multi-iteration mode does not accept a positional `<issue>` argument. Targeting a specific issue is a single-run operation.

**Iteration stop conditions**

In multi-iteration mode, Ralph stops and exits after whichever of these occurs first:

- `--count <n>` iterations have completed (whether successful or escalated)
- `--loop` is set and the queue is empty or all remaining issues are dependency-blocked
- Any iteration exits due to a pre-flight failure (not an escalation — a configuration error)
- The operator sends SIGINT (Ctrl-C): the current iteration completes cleanly before Ralph exits

An escalation in one iteration does not stop the loop. Ralph posts the escalation comment, labels the issue `needs-human`, and moves on to the next eligible issue. This allows the queue to drain past a blocked issue rather than stalling. If this behavior is not desired, use `--stop-on-escalation`.

**Progress output in multi-iteration mode**

Ralph prints a header line before each iteration:

```
━━━ Iteration 1 of 3 — issue #38: Add user authentication model ━━━
...run output...

━━━ Iteration 2 of 3 — issue #41: Implement JWT middleware ━━━
...run output...
```

At the end of a multi-iteration run, Ralph prints a summary:

```
Loop complete — 3 iterations

  #38  success    4m 12s   CI: green
  #41  escalated  1m 03s   CI: —      (needs-human)
  #47  success    5m 51s   CI: green
```

**Flags**

| Flag | Description |
|---|---|
| `--loop` | Run continuously until the queue is empty or all remaining issues are dependency-blocked. Incompatible with `<issue>` and `--count`. |
| `--count <n>` | Run exactly `n` iterations, then exit. Incompatible with `<issue>` and `--loop`. |
| `--stop-on-escalation` | Exit immediately if any iteration escalates, rather than continuing to the next issue. Exit code is 1. |
| `--dry-run` | Assemble context and print the initial prompt that would be sent to the agent, then exit without spawning the agent or writing the lockfile. In multi-iteration mode, previews the first iteration only. |
| `--no-notify` | Suppress per-iteration completion notifications. A single summary notification is still sent at the end of a multi-iteration run unless `--no-notify` is combined with `--loop` or `--count`. |
| `--retries <n>` | Override `max_retries` from `ralph.json` for all iterations in this invocation. |
| `--poll-interval <duration>` | Override the signal file poll interval. Accepts duration strings: `5s`, `10s`, `1m`. Default: `5s`. |

**Argument and flag compatibility**

| Invocation | Behavior |
|---|---|
| `ralph run` | One iteration, next eligible issue |
| `ralph run 42` | One iteration, issue #42 specifically |
| `ralph run --count 5` | Up to 5 iterations, queue order |
| `ralph run --loop` | Continuous until queue empty |
| `ralph run 42 --dry-run` | Preview prompt for issue #42, no agent |
| `ralph run --loop --dry-run` | Preview prompt for next issue only, no agent |
| `ralph run 42 --loop` | Invalid — exit 1 with explanation |
| `ralph run 42 --count 3` | Invalid — exit 1 with explanation |

**Exit codes**

| Code | Meaning |
|---|---|
| 0 | All iterations completed; at least one succeeded; queue drained or count reached |
| 1 | Pre-flight failure, or `--stop-on-escalation` triggered, or all iterations escalated |
| 2 | No eligible issues found before any iteration ran (queue empty or all dependency-blocked) |

In multi-iteration mode, exit code 0 does not mean every iteration succeeded — it means the run completed normally and at least one issue was processed. Check `ralph log` for per-issue outcomes. Exit code 1 in multi-iteration mode means the run was interrupted, not merely that some issues escalated.

---

### 13.4 `ralph issue`

Subcommand group for issue management. Provides helpers for creating and inspecting issues without leaving the terminal. Does not replace GitHub's issue UI — it accelerates the spec-to-issue step.

---

#### `ralph issue create`

Drafts a GitHub issue from a spec file, following the issue template in Section 4. Opens the draft for review before submission.

**Usage**
```
ralph issue create --spec <path> [flags]
```

**Behavior**

1. Reads the spec file at the given path.
2. Generates a draft issue body: extracts or infers a summary from the spec, produces acceptance criteria from the spec's requirements language, and populates the `Specs` section with the given path.
3. Opens the draft in the operator's `$EDITOR` for review and editing before submission.
4. On save and exit from the editor, creates the issue via `gh issue create`.
5. Does not apply the `ralph-ready` label. The operator applies it when the issue is ready to enter the queue (see Section 13.4.2).
6. Prints the created issue number and URL.

**Flags**

| Flag | Description |
|---|---|
| `--spec <path>` | Path to the spec file to generate the issue from. Required. |
| `--title <string>` | Issue title. If omitted, inferred from the spec filename or first heading. |
| `--depends-on <numbers>` | Comma-separated issue numbers to populate the `Depends On` section. Example: `--depends-on 38,41`. |
| `--no-edit` | Skip opening the editor. Submit the generated draft directly. Use only when the generated output is known to be correct. |
| `--milestone <name>` | Assign the created issue to a milestone by name. |

---

#### `ralph issue ready <number>`

Applies the `ralph-ready` label to an issue, making it eligible for selection by `ralph run`.

**Usage**
```
ralph issue ready <number>
```

**Behavior**

1. Confirms the issue is open.
2. Confirms the issue body conforms to the template (has `Summary`, `Specs`, and `Acceptance Criteria` sections). Warns if any section is missing but does not block.
3. Confirms all spec files listed in the `Specs` section exist on disk. Warns if any are missing but does not block — the operator may be creating them in the same session.
4. Applies the `ralph-ready` label.
5. Prints confirmation.

**Error conditions**

- Issue not found → exit 1
- Issue already closed → exit 1
- Issue already labeled `ralph-ready` → prints a notice, exits 0

---

#### `ralph issue list`

Lists issues currently in the ralph queue, with their eligibility status.

**Usage**
```
ralph issue list [flags]
```

**Behavior**

Lists all open `ralph-ready` issues. For each issue, shows: number, title, eligibility (eligible or blocked on dependencies), and the `Depends On` issues if blocked.

Example output:
```
#38  Add user authentication model         eligible
#41  Implement JWT middleware               blocked (depends on #38)
#47  Add refresh token endpoint            blocked (depends on #41)
#52  Write integration tests for auth      blocked (depends on #38, #41, #47)
```

Issues are listed in the order they would be selected by `ralph run`.

**Flags**

| Flag | Description |
|---|---|
| `--json` | Output as a JSON array. |
| `--all` | Include issues labeled `needs-human` in addition to `ralph-ready` issues. |

---

### 13.5 `ralph status`

Reports the current state of the system: whether a run is in progress, which issue is being worked, and CI status on any open ralph-managed PRs.

**Usage**
```
ralph status [flags]
```

**Behavior**

If a lockfile is present and the recorded process is running:
```
● Running
  Issue:   #41 — Implement JWT middleware
  Branch:  issue/41-implement-jwt-middleware
  Started: 14 minutes ago
  Stage:   agent running
```

If no lockfile is present:
```
○ Idle

Open ralph PRs:
  #83  issue/38-add-user-auth-model         CI: ✓ green   (awaiting review)
  #84  issue/41-implement-jwt-middleware     CI: ✗ failing (2 retries remaining)
```

If no lockfile and no open ralph PRs:
```
○ Idle — queue has 3 eligible issues. Run `ralph run` to start.
```

**Flags**

| Flag | Description |
|---|---|
| `--json` | Output as JSON. |
| `--watch` | Refresh output every 5 seconds until interrupted. |

---

### 13.6 `ralph log`

Displays the structured run log for past runs (see Section 10.2).

**Usage**
```
ralph log [flags]
```

**Behavior**

By default, prints a summary of the last 10 runs: issue number, outcome (success / escalated), duration, and CI result.

Example output:
```
RUN   ISSUE  OUTCOME    DURATION  CI
────  ─────  ─────────  ────────  ──────
#009  #38    success    4m 12s    green
#008  #35    escalated  1m 03s    —
#007  #33    success    6m 44s    green
#006  #31    success    3m 58s    green
```

**Flags**

| Flag | Description |
|---|---|
| `--run <n>` | Show full detail for a specific run number, including the assembled context summary, agent signal received, CI check results, and escalation body if applicable. |
| `--issue <number>` | Filter to runs for a specific issue number. |
| `--limit <n>` | Number of runs to show. Default: 10. |
| `--json` | Output as JSON. |

---

### 13.7 Global Flags

These flags are accepted by all commands.

| Flag | Description |
|---|---|
| `--config <path>` | Path to `ralph.json`. Defaults to `./ralph.json` in the current directory. |
| `--verbose` | Print detailed step-by-step output. Useful for debugging context assembly and agent lifecycle events. |
| `--no-color` | Disable color and emoji in output. Automatically set if stdout is not a TTY. |
| `--help` | Print usage for the command. |
| `--version` | Print the ralph version and exit. |

---

### 13.8 Configuration File (`ralph.json`)

The configuration file lives at the repo root and governs default behavior for all commands. Flags passed at invocation time override config file values for that invocation only.

```json
{
  "issue_filter": {
    "labels": ["ralph-ready"],
    "state": "open",
    "assignee": "none"
  },
  "branch_convention": "issue/{number}-{slug}",
  "base_branch": "main",
  "max_retries": 3,
  "poll_interval": "5s",
  "context": {
    "always_include": ["AGENTS.md"],
    "spec_source": "issue_body"
  },
  "escalation": {
    "on_ci_failure": true,
    "on_scope_violation": true,
    "on_agent_blocked": true
  },
  "notify": {
    "provider": "chime",
    "on_success": true,
    "on_escalation": true
  },
  "log": {
    "path": ".ralph/runs/",
    "retain": 50
  }
}
```

All fields are optional. Missing fields take the defaults shown above. Unknown fields are ignored with a warning rather than causing a hard failure, to allow forward compatibility.

---

### 13.9 Command Summary

| Command | Description |
|---|---|
| `ralph init` | Scaffold Ralph infrastructure into a new repo |
| `ralph validate` | Pre-flight check; confirm system is ready to run |
| `ralph run` | Execute one iteration against the next eligible issue |
| `ralph run <issue>` | Execute one iteration against a specific issue number |
| `ralph run --count <n>` | Execute exactly n iterations in queue order |
| `ralph run --loop` | Run continuously until the queue is empty |
| `ralph run --dry-run` | Preview the prompt that would be sent to the agent |
| `ralph run --loop --stop-on-escalation` | Run continuously, halt on first escalation |
| `ralph issue create` | Draft a GitHub issue from a spec file |
| `ralph issue ready <n>` | Apply `ralph-ready` label to an issue |
| `ralph issue list` | List queued issues and their eligibility |
| `ralph status` | Show current run state and open PR status |
| `ralph log` | Show history of past runs |
| `ralph log --run <n>` | Show full detail for a specific run |