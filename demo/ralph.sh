#!/usr/bin/env bash
#
# ralph.sh — solve a list of GitHub issues one-by-one with codex, ralph-style.
#
# Usage:
#   ./ralph.sh <prompt.md> <issue-number> [issue-number ...]
#   ./ralph.sh <prompt.md> --issues-file issues.txt
#
# The prompt file is a template. Anywhere it contains the token {{ISSUE}}
# it gets replaced with the resolved issue body/URL for that iteration.
# If the token is absent, the issue reference is appended to the end
# of the prompt instead (so you don't have to edit templates to use this).
#
# Requires: gh (GitHub CLI, authenticated), codex CLI on PATH.
#
# Env vars:
#   REPO                     owner/repo (default: gh's cwd detection)
#   MAX_RETRIES              retry count for the solve step (default: 1)
#   COMMIT_MAX_RETRIES       retry count for the commit step (default: 1)
#   SIGNAL_DIR               where .done/.escalate signals + logs go (default: .ralph)
#   SOLVE_MODEL              -m/--model for the issue-solving step
#   SOLVE_REASONING_EFFORT   reasoning effort for the solving step (low/medium/high),
#                            passed via -c model_reasoning_effort=<value>
#   COMMIT_MODEL             -m/--model for the commit step
#   COMMIT_REASONING_EFFORT  reasoning effort for the commit step, same mechanism
#   CODEX_SANDBOX            -s/--sandbox mode for both steps, e.g. workspace-write
#                            (unset = whatever ~/.codex/config.toml already specifies)
#
# Example: cheap/fast model for commits, stronger model + reasoning for solving:
#   SOLVE_MODEL=o3 SOLVE_REASONING_EFFORT=high \
#   COMMIT_MODEL=o4-mini COMMIT_REASONING_EFFORT=low \
#   CODEX_SANDBOX=workspace-write \
#   ./ralph.sh prompt.md 101 102 103

set -euo pipefail

REPO="${REPO:-}"          # optional: owner/repo, defaults to gh's repo-in-cwd detection
MAX_RETRIES="${MAX_RETRIES:-1}"
SIGNAL_DIR="${SIGNAL_DIR:-.ralph}"
COMMIT_MAX_RETRIES="${COMMIT_MAX_RETRIES:-1}"
CODEX_SANDBOX="${CODEX_SANDBOX:-}"

# Model/reasoning config, configurable independently per step.
# Leave a var empty/unset to fall back to codex's own defaults.
SOLVE_MODEL="${SOLVE_MODEL:-}"
SOLVE_REASONING_EFFORT="${SOLVE_REASONING_EFFORT:-}"
COMMIT_MODEL="${COMMIT_MODEL:-}"
COMMIT_REASONING_EFFORT="${COMMIT_REASONING_EFFORT:-}"

# Build codex CLI flag arrays from the above.
# NOTE: `codex exec --help` (checked against the installed version) has
# no --reasoning-effort flag — reasoning effort is set via the -c config
# override (model_reasoning_effort), not a dedicated flag. -m/--model is
# a real flag. -s/--sandbox is also a real flag, applied to both steps
# via CODEX_SANDBOX if set.
sandbox_flags=()
[[ -n "$CODEX_SANDBOX" ]] && sandbox_flags+=(-s "$CODEX_SANDBOX")

solve_codex_flags=("${sandbox_flags[@]}")
[[ -n "$SOLVE_MODEL" ]] && solve_codex_flags+=(-m "$SOLVE_MODEL")
[[ -n "$SOLVE_REASONING_EFFORT" ]] && solve_codex_flags+=(-c "model_reasoning_effort=\"${SOLVE_REASONING_EFFORT}\"")

commit_codex_flags=("${sandbox_flags[@]}")
[[ -n "$COMMIT_MODEL" ]] && commit_codex_flags+=(-m "$COMMIT_MODEL")
[[ -n "$COMMIT_REASONING_EFFORT" ]] && commit_codex_flags+=(-c "model_reasoning_effort=\"${COMMIT_REASONING_EFFORT}\"")

# Fixed prompt for the dedicated commit step. Kept separate from the
# solve-issue prompt template on purpose — this call only ever sees the
# diff, never the issue body, so it can't get distracted into "fixing"
# anything else. Swap this out for your commit-plan skill later.
read -r -d '' COMMIT_PROMPT_TEMPLATE <<'EOF' || true
You are only responsible for committing the current working tree changes.
Do not modify any files. Do not make further code changes.

1. Run `git add -A`.
2. Write a clear, conventional commit message summarizing the actual
   diff (not the issue description) — what changed and why, in the
   imperative mood (e.g. "Fix null check in parser").
3. Reference "Issue #{{ISSUE_NUM}}" in the commit message footer.
4. Commit with that message.

If there is nothing staged to commit, say so explicitly and exit
without creating an empty commit.
EOF

usage() {
  echo "Usage: $0 <prompt.md> <issue-number> [issue-number ...]" >&2
  echo "       $0 <prompt.md> --issues-file <file>" >&2
  exit 1
}

[[ $# -ge 2 ]] || usage

PROMPT_FILE="$1"; shift
[[ -f "$PROMPT_FILE" ]] || { echo "Prompt file not found: $PROMPT_FILE" >&2; exit 1; }

if [[ "$1" == "--issues-file" ]]; then
  ISSUES_FILE="$2"
  [[ -f "$ISSUES_FILE" ]] || { echo "Issues file not found: $ISSUES_FILE" >&2; exit 1; }
  mapfile -t ISSUES < <(grep -Eo '[0-9]+' "$ISSUES_FILE")
else
  ISSUES=("$@")
fi

[[ ${#ISSUES[@]} -gt 0 ]] || { echo "No issue numbers provided." >&2; exit 1; }

mkdir -p "$SIGNAL_DIR"
PROMPT_TEMPLATE="$(cat "$PROMPT_FILE")"

gh_repo_flag=()
[[ -n "$REPO" ]] && gh_repo_flag=(--repo "$REPO")

for issue in "${ISSUES[@]}"; do
  echo "=============================================="
  echo "Ralph loop: issue #${issue}"
  echo "=============================================="

  # Pull title + body so the model has real content, not just a number.
  issue_json="$(gh issue view "$issue" "${gh_repo_flag[@]}" --json title,body,url)"
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

    # codex prints its own output; tee lets us keep a per-issue log too
    # while still showing everything live in stdout.
    if codex exec "${solve_codex_flags[@]}" "$final_prompt" 2>&1 | tee "${SIGNAL_DIR}/issue-${issue}.log"; then
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

  # --- Separate, dedicated commit step ---------------------------------
  # Runs as its own codex process so committing isn't left riding on
  # whatever the solve step happened to remember to do. This step only
  # ever sees the diff, not the issue text.
  commit_prompt="${COMMIT_PROMPT_TEMPLATE//\{\{ISSUE_NUM\}\}/$issue}"

  commit_done_signal="${SIGNAL_DIR}/issue-${issue}.commit.done"
  commit_escalate_signal="${SIGNAL_DIR}/issue-${issue}.commit.escalate"
  rm -f "$commit_done_signal" "$commit_escalate_signal"

  commit_attempt=1
  commit_success=0
  while [[ $commit_attempt -le $COMMIT_MAX_RETRIES ]]; do
    echo "--- commit attempt ${commit_attempt}/${COMMIT_MAX_RETRIES} for issue #${issue} ---"

    if codex exec "${commit_codex_flags[@]}" "$commit_prompt" 2>&1 | tee "${SIGNAL_DIR}/issue-${issue}.commit.log"; then
      commit_success=1
      break
    fi

    commit_attempt=$((commit_attempt + 1))
  done

  if [[ $commit_success -eq 1 ]]; then
    touch "$done_signal" "$commit_done_signal"
    echo "Issue #${issue}: done (solved + committed)"
  else
    touch "$commit_escalate_signal"
    echo "Issue #${issue}: solved but commit escalated after ${COMMIT_MAX_RETRIES} attempt(s)" >&2
  fi
done

echo "Ralph loop complete. Signals in ${SIGNAL_DIR}/"