#!/usr/bin/env bash
#
# ralph-pi.sh — solve a list of GitHub issues one-by-one with pi, ralph-style.
#
# Usage:
#   ./ralph-pi.sh <prompt.md> <issue-number> [issue-number ...]
#   ./ralph-pi.sh <prompt.md> --issues-file issues.txt
#
# The prompt file is a template. Anywhere it contains the token {{ISSUE}}
# it gets replaced with the resolved issue body/URL for that iteration.
# If the token is absent, the issue reference is appended to the end
# of the prompt instead (so you don't have to edit templates to use this).
#
# Requires: gh (GitHub CLI, authenticated), pi CLI on PATH.
#
# Env vars:
#   REPO                     owner/repo (default: gh's cwd detection)
#   MAX_RETRIES              retry count for the solve step (default: 1)
#   COMMIT_MAX_RETRIES       retry count for the commit step (default: 1)
#   SIGNAL_DIR               where .done/.escalate signals + logs go (default: .ralph)
#   SOLVE_MODEL              --model for the issue-solving step
#   SOLVE_THINKING           --thinking for the solving step
#                            (off/minimal/low/medium/high/xhigh)
#   COMMIT_MODEL             --model for the commit step
#   COMMIT_THINKING          --thinking for the commit step
#   RALPH_PI_SANDBOX         Pi tool mode: read-only or workspace-write (default).
#                            read-only enables only Pi's read-only tools. workspace-write
#                            enables Pi's default tools, but is NOT a filesystem sandbox.
#   RALPH_SHOW_TRACE         stream Pi output and diagnostics to the terminal too (default: 0)
#
# Example: cheap/fast model for commits, stronger model + reasoning for solving:
#   SOLVE_MODEL=openai/o3 SOLVE_THINKING=high \
#   COMMIT_MODEL=openai/o4-mini COMMIT_THINKING=low \
#   RALPH_PI_SANDBOX=workspace-write \
#   ./ralph-pi.sh prompt.md 101 102 103

set -euo pipefail

REPO="${REPO:-}"          # optional: owner/repo, defaults to gh's repo-in-cwd detection
MAX_RETRIES="${MAX_RETRIES:-1}"
SIGNAL_DIR="${SIGNAL_DIR:-.ralph}"
COMMIT_MAX_RETRIES="${COMMIT_MAX_RETRIES:-1}"
# Pi does not provide Codex-style filesystem sandboxing. This setting controls
# only Pi's built-in tool allowlist; RALPH_PI_SANDBOX is deliberately not
# inherited from an unrelated PI_SANDBOX environment variable.
PI_SANDBOX="${RALPH_PI_SANDBOX:-workspace-write}"

case "$PI_SANDBOX" in
  read-only|workspace-write) ;;
  *)
    echo "Invalid RALPH_PI_SANDBOX: $PI_SANDBOX (use read-only or workspace-write)" >&2
    exit 1
    ;;
esac

# Model/reasoning config, configurable independently per step.
# Leave a var empty/unset to fall back to pi's own defaults.
SOLVE_MODEL="${SOLVE_MODEL:-}"
SOLVE_THINKING="${SOLVE_THINKING:-}"
COMMIT_MODEL="${COMMIT_MODEL:-}"
COMMIT_THINKING="${COMMIT_THINKING:-}"
RALPH_SHOW_TRACE="${RALPH_SHOW_TRACE:-0}"

case "$SOLVE_THINKING" in
  ""|off|minimal|low|medium|high|xhigh) ;;
  *)
    echo "Invalid SOLVE_THINKING: $SOLVE_THINKING" >&2
    exit 1
    ;;
esac

case "$COMMIT_THINKING" in
  ""|off|minimal|low|medium|high|xhigh) ;;
  *)
    echo "Invalid COMMIT_THINKING: $COMMIT_THINKING" >&2
    exit 1
    ;;
esac

case "$RALPH_SHOW_TRACE" in
  0|1) ;;
  *)
    echo "Invalid RALPH_SHOW_TRACE: $RALPH_SHOW_TRACE (use 0 or 1)" >&2
    exit 1
    ;;
esac

# Build pi CLI flag arrays from the above.
# Run each call without persistent Pi session state and force text output so
# the commit-message markers can be parsed reliably.
pi_base_flags=(--no-session --mode text)
solve_pi_flags=("${pi_base_flags[@]}")
if [[ "$PI_SANDBOX" == "read-only" ]]; then
  solve_pi_flags+=(--tools "read,grep,find,ls")
fi
[[ -n "$SOLVE_MODEL" ]] && solve_pi_flags+=(--model "$SOLVE_MODEL")
[[ -n "$SOLVE_THINKING" ]] && solve_pi_flags+=(--thinking "$SOLVE_THINKING")

commit_pi_flags=("${pi_base_flags[@]}")
if [[ "$PI_SANDBOX" == "read-only" ]]; then
  commit_pi_flags+=(--tools "read,grep,find,ls")
fi
[[ -n "$COMMIT_MODEL" ]] && commit_pi_flags+=(--model "$COMMIT_MODEL")
[[ -n "$COMMIT_THINKING" ]] && commit_pi_flags+=(--thinking "$COMMIT_THINKING")

# Fixed prompt for the dedicated commit-message step. Kept separate from the
# solve-issue prompt template on purpose — this call only ever sees the diff,
# never the issue body, so it can't get distracted into "fixing" anything else.
read -r -d '' COMMIT_PROMPT_TEMPLATE <<'EOF' || true
# Task

You are responsible for preparing a commit message for the current working
tree changes. Do not modify files, stage files, or create a commit.

1. Inspect the actual diff with `git diff` and `git status`.
2. Write a clear, conventional commit message summarizing the actual
   diff (not the issue description) — what changed and why, in the
   imperative mood (e.g. "Fix null check in parser").

If there are no working tree changes, leave the marker body empty and do not
invent a commit message.

Your final response must contain exactly one message between these markers,
with no Markdown fences and no commentary inside the markers:

BEGIN_COMMIT_MESSAGE
<complete commit message>
END_COMMIT_MESSAGE

## Commit Format

Use Conventional Commit headers. Choose one allowed type: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, or `revert`.
 
Choose a consistent lowercase scope based on the affected service or repository domain. Examples include `android`, `api`, `web`, `user-service`, `android/auth`, `api/config`, and `db/migrations`. Nested scopes are allowed when they identify a meaningful subsystem within a broader area. Omit the scope only when the change is repo-wide or no meaningful scope applies. Use an agent-focused documentation scope such as `docs/agent` for `AGENTS.md` changes.
 
Write a commit body of 1–6 sentences. Use imperative, present-tense sentences and standard sentence-case capitalization with ending punctuation. Use newlines only to separate the header, paragraphs, and footer; do not hard-wrap ordinary body text.
 
End every commit message with this exact footer, after one blank line:
 
`Co-Authored-By: Pi Coding Agent <noreply@pi.dev>`
 
### Header
 
- Use Conventional Commit format: `type(scope): description`.
- Make the header entirely lowercase, except for acronyms or proper nouns that must remain capitalized inside the scope.
- Keep the header free of a final period.
- Write the description in imperative, present tense, as a command to the codebase: use `add support`, `fix parsing`, or `update docs`, not `added support`, `fixes parsing`, or `updated docs`.
- For a breaking change, append `!` directly after the closing scope parenthesis, as in `feat(api/config)!: drop support for v1 endpoints`.

### Body
 
- Write 1–6 sentences based on the commit's complexity.
- Use imperative, present-tense sentences and standard sentence-case capitalization with ending punctuation.
- Do not use Markdown bullets or lists. If multiple distinct points need separate treatment, use at most two paragraphs separated by one blank line.
- For a breaking change, start the body with `BREAKING CHANGE:` followed by a clear description of what breaks and how consumers should adapt.
EOF

usage() {
  echo "Usage: $0 <prompt.md> <issue-number> [issue-number ...]" >&2
  echo "       $0 <prompt.md> --issues-file <file>" >&2
  exit 1
}

[[ $# -ge 2 ]] || usage

for required_command in gh pi jq; do
  command -v "$required_command" >/dev/null 2>&1 || {
    echo "Required command not found on PATH: $required_command" >&2
    exit 1
  }
done

PROMPT_FILE="$1"; shift
[[ -f "$PROMPT_FILE" ]] || { echo "Prompt file not found: $PROMPT_FILE" >&2; exit 1; }

if [[ "$1" == "--issues-file" ]]; then
  [[ $# -eq 2 ]] || usage
  ISSUES_FILE="$2"
  [[ -r "$ISSUES_FILE" ]] || { echo "Issues file is not readable: $ISSUES_FILE" >&2; exit 1; }

  # macOS ships Bash 3.2, which does not have `mapfile`.
  ISSUES=()
  while IFS= read -r issue; do
    [[ -n "$issue" ]] && ISSUES+=("$issue")
  done < <(grep -Eo '[0-9]+' "$ISSUES_FILE" || true)
else
  ISSUES=("$@")
fi

[[ ${#ISSUES[@]} -gt 0 ]] || { echo "No issue numbers provided." >&2; exit 1; }
for issue in "${ISSUES[@]}"; do
  [[ "$issue" =~ ^[0-9]+$ ]] || { echo "Invalid issue number: $issue" >&2; exit 1; }
done

mkdir -p "$SIGNAL_DIR"
PROMPT_TEMPLATE="$(cat "$PROMPT_FILE")"

for issue in "${ISSUES[@]}"; do
  echo "=============================================="
  echo "Ralph loop: issue #${issue}"
  echo "=============================================="

  # Pull title + body so the model has real content, not just a number.
  if [[ -n "$REPO" ]]; then
    issue_json="$(gh issue view "$issue" --repo "$REPO" --json title,body,url)"
  else
    issue_json="$(gh issue view "$issue" --json title,body,url)"
  fi
  issue_title="$(jq -r '.title' <<<"$issue_json")"
  issue_body="$(jq -r '.body' <<<"$issue_json")"
  issue_url="$(jq -r '.url' <<<"$issue_json")"

  issue_block=$(cat <<EOF
### GitHub Issue #${issue}: ${issue_title}
${issue_url}

${issue_body}
EOF
)

  # Embed into the prompt: replace {{ISSUE}} if present, else append.
  if grep -q '{{ISSUE}}' <<<"$PROMPT_TEMPLATE"; then
    final_prompt="${PROMPT_TEMPLATE//\{\{ISSUE\}\}/$issue_block}"
  else
    final_prompt="${PROMPT_TEMPLATE}

${issue_block}"
  fi

  done_signal="${SIGNAL_DIR}/issue-${issue}.done"
  escalate_signal="${SIGNAL_DIR}/issue-${issue}.escalate"
  rm -f "$done_signal" "$escalate_signal"

  attempt=1
  success=0
  while [[ $attempt -le $MAX_RETRIES ]]; do
    echo "--- attempt ${attempt}/${MAX_RETRIES} for issue #${issue} ---"

    solve_log="${SIGNAL_DIR}/issue-${issue}.attempt-${attempt}.log"
    solve_summary="${SIGNAL_DIR}/issue-${issue}.attempt-${attempt}.summary.md"
    if [[ "$RALPH_SHOW_TRACE" == "1" ]]; then
      if pi "${solve_pi_flags[@]}" -p "$final_prompt" 2>&1 | tee "$solve_summary" "$solve_log"; then
        solve_exit=0
      else
        solve_exit=$?
      fi
    else
      if pi "${solve_pi_flags[@]}" -p "$final_prompt" >"$solve_summary" 2>"$solve_log"; then
        solve_exit=0
      else
        solve_exit=$?
      fi
    fi
    if [[ $solve_exit -eq 0 ]]; then
      echo "--- Pi summary for issue #${issue} ---"
      sed -n '1,160p' "$solve_summary" 2>/dev/null || true
      success=1
      break
    fi

    attempt=$((attempt + 1))
  done

  if [[ $success -ne 1 ]]; then
    touch "$escalate_signal"
    echo "Issue #${issue}: escalated after ${MAX_RETRIES} attempt(s)" >&2
    continue
  fi

  # Pi's read-only tool allowlist deliberately excludes bash, so it cannot
  # inspect Git state for the commit-message step. Treat this as a completed
  # review pass rather than an issue implementation or a failed commit.
  if [[ "$PI_SANDBOX" == "read-only" ]]; then
    touch "$done_signal"
    echo "Issue #${issue}: read-only review complete; no changes committed"
    continue
  fi

  # --- Separate, dedicated commit-message step --------------------------
  # Runs as its own Pi process so message generation isn't left riding on
  # whatever the solve step happened to remember to do. The host shell below
  # performs the actual Git operations after Pi exits.
  commit_prompt="${COMMIT_PROMPT_TEMPLATE//\{\{ISSUE_NUM\}\}/$issue}"

  commit_message_done_signal="${SIGNAL_DIR}/issue-${issue}.commit-message.done"
  commit_done_signal="${SIGNAL_DIR}/issue-${issue}.commit.done"
  commit_escalate_signal="${SIGNAL_DIR}/issue-${issue}.commit.escalate"
  commit_message_file="${SIGNAL_DIR}/issue-${issue}.commit-message.txt"
  rm -f "$commit_message_done_signal" "$commit_done_signal" "$commit_escalate_signal" "$commit_message_file"

  commit_attempt=1
  commit_success=0
  while [[ $commit_attempt -le $COMMIT_MAX_RETRIES ]]; do
    echo "--- commit attempt ${commit_attempt}/${COMMIT_MAX_RETRIES} for issue #${issue} ---"

    commit_log="${SIGNAL_DIR}/issue-${issue}.commit.attempt-${commit_attempt}.log"
    commit_summary="${SIGNAL_DIR}/issue-${issue}.commit.attempt-${commit_attempt}.summary.md"
    if [[ "$RALPH_SHOW_TRACE" == "1" ]]; then
      if pi "${commit_pi_flags[@]}" -p "$commit_prompt" 2>&1 | tee "$commit_summary" "$commit_log"; then
        commit_exit=0
      else
        commit_exit=$?
      fi
    else
      if pi "${commit_pi_flags[@]}" -p "$commit_prompt" >"$commit_summary" 2>"$commit_log"; then
        commit_exit=0
      else
        commit_exit=$?
      fi
    fi
    if [[ $commit_exit -eq 0 ]]; then
      if commit_message="$(awk '
        /BEGIN_COMMIT_MESSAGE/ { in_message=1; next }
        /END_COMMIT_MESSAGE/ { found=1; in_message=0; next }
        in_message { print }
        END { if (!found) exit 1 }
      ' "$commit_summary")" && [[ -n "$commit_message" ]]; then
        printf '%s\n' "$commit_message" >"$commit_message_file"
        echo "--- Commit message for issue #${issue} ---"
        sed -n '1,80p' "$commit_message_file"
        commit_success=1
        break
      fi
      echo "Commit step did not produce a valid marked commit message." >&2
    fi

    commit_attempt=$((commit_attempt + 1))
  done

  if [[ $commit_success -eq 1 ]]; then
    touch "$commit_message_done_signal"
    echo "--- Host commit for issue #${issue} ---"
    if ! git add -A; then
      echo "Issue #${issue}: solved but host git add failed" >&2
      touch "$commit_escalate_signal"
      continue
    fi

    if git diff --cached --quiet; then
      touch "$done_signal" "$commit_done_signal"
      echo "Issue #${issue}: solved; no changes to commit"
    elif git commit -F "$commit_message_file"; then
      touch "$done_signal" "$commit_done_signal"
      echo "Issue #${issue}: done (solved + committed)"
    else
      echo "Issue #${issue}: solved but host git commit failed" >&2
      touch "$commit_escalate_signal"
    fi
  else
    touch "$commit_escalate_signal"
    echo "Issue #${issue}: solved but commit escalated after ${COMMIT_MAX_RETRIES} attempt(s)" >&2
  fi
done

echo "Ralph loop complete. Signals in ${SIGNAL_DIR}/"
