# Agent Guidelines & Skill Contracts

This repository contains the Ralph automated development system. All autonomous coding agents working in this repository must strictly adhere to the instructions and contracts outlined below.

---

## 1. Environment & Build Commands

- **Language:** Go 1.26+
- **Test Command:** `go test -v ./...`
- **Lint / Format:** `go vet ./...` and `gofmt -s -w .`

---

## 2. Phase A: Implementation Phase Rules (Commit & Handoff)

During the issue implementation phase (Phase A), you must follow these rules:

1. **Logical Chunking & Testing:**
   - Implement your changes in logical, verifiable units.
   - Run the full test suite (`go test -v ./...`) after completing each logical chunk of work to verify that no regressions have been introduced.
2. **Commit Format:**
   - Format all commit messages using Conventional Commits: `<type>(<scope>): <description>`.
   - Append `(#<issue_number>)` to the commit description line (e.g., `feat(auth): add jwt validation middleware (#42)`).
3. **Handoff Documentation:**
   - Before completing your session and writing `ralph.done`, you **MUST** append a brief, bulleted summary of what you implemented, any new files created, or major architectural decisions made to `.ralph-handoff.md` at the repository root.
   - This handoff document is critical because subsequent agent sessions will rely directly on it to understand the evolving architecture.
4. **Completion / Escalation Signals:**
   - When the issue is fully implemented, tests pass, and `.ralph-handoff.md` is updated, write `ralph.done` containing the text `success` to the repository root and stop.
   - If you are genuinely blocked (e.g., due to an ambiguous or missing specification), write `ralph.escalate` to the repository root containing one paragraph describing the blocker, and stop.

---

## 3. Phase B: Draft-PR Skill (Unified Mode)

During the Pull Request drafting phase (Phase B), you operate under a dedicated, isolated session:

1. **Scope:**
   - Your **only** task in this session is to write the Pull Request description. Do not write or modify implementation code.
2. **Output File:**
   - Write your Pull Request markdown document to `.ralph/draft-pr.md`.
3. **Content Requirements:**
   - **Title Line:** The first line must be a high-level title summarizing the unified scope across all implemented issues (e.g., `# Unified PR: Authentication & Refresh Token Pipeline`).
   - **Linked Issues:** A section explicitly listing `Closes #<issue_number>` for every issue in the batch.
   - **The "How":** A narrative, plain-English section explaining what was changed and why across the entire batch, referencing `.ralph-handoff.md` as your primary source of truth.
4. **Pushing & Completion:**
   - After writing `.ralph/draft-pr.md`, push the branch to the remote repository:
     ```bash
     git push -u origin HEAD
     ```
   - Once the push succeeds, write `ralph.done` containing `success` to the repository root and exit.
   - If blocked, write `ralph.escalate` describing the blocker to the repository root and exit.

---

## 4. Phase C: CI Fix Skill

During CI fix sessions (Phase C), you respond to automated CI build or test failures:

1. **Scope:**
   - Inspect the provided CI failure logs and the architectural context in `.ralph-handoff.md`.
   - Diagnose the root cause of the failure, fix the offending code or tests in the workspace, and run `go test -v ./...` locally to ensure everything passes cleanly.
2. **Commit & Push:**
   - Amend the previous commit or create a new commit documenting the fix.
   - Push the branch to the remote repository:
     ```bash
     git push -u origin HEAD
     # Or if amending:
     git push --force-with-lease -u origin HEAD
     ```
3. **Completion:**
   - Once pushed, write `ralph.done` containing `success` to the repository root and exit.
   - If unresolvable or blocked, write `ralph.escalate` describing the blocker to the repository root and exit.
