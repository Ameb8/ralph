#!/bin/bash
set -e

# ralph-v1.sh
# Unified PR V1 implementation of Ralph.
# Usage: ./ralph-v1.sh <issue1> <issue2> ...

if [ "$#" -eq 0 ]; then
    echo "Usage: $0 <issue1> <issue2> ..."
    exit 1
fi

ISSUES=("$@")
BRANCH_NAME="ralph/unified-$(IFS=-; echo "${ISSUES[*]}")"

echo "=== Ralph V1 Orchestrator ==="
echo "Target issues: ${ISSUES[*]}"
echo "Branch: $BRANCH_NAME"

# 1. Pre-flight
if ! command -v gh &> /dev/null; then
    echo "Error: gh CLI not installed."
    exit 1
fi
if [ ! -f "AGENTS.md" ]; then
    echo "Warning: AGENTS.md not found. Agent may lack repo context."
fi

# 2. Setup branch
git checkout main
git pull origin main
git checkout -b "$BRANCH_NAME"

# 3. Context Assembly
PROMPT_FILE=".ralph-prompt.md"
cat << 'PROMPT' > "$PROMPT_FILE"
You are operating as the Ralph agent. 
Below is your combined context for a unified set of issues.
Implement all issues, commit your work in logical chunks, run tests, and write "success" to `ralph.done` when fully complete. If genuinely blocked, write your blocker to `ralph.escalate`.
Do not ask for human input.

# Repository Conventions (AGENTS.md)
PROMPT

if [ -f "AGENTS.md" ]; then
    cat AGENTS.md >> "$PROMPT_FILE"
fi

echo -e "\n# Target Issues" >> "$PROMPT_FILE"

declare -A SPECS_TO_READ

for ISSUE_ID in "${ISSUES[@]}"; do
    echo "Fetching issue #$ISSUE_ID..."
    ISSUE_JSON=$(gh issue view "$ISSUE_ID" --json title,body)
    TITLE=$(echo "$ISSUE_JSON" | jq -r .title)
    BODY=$(echo "$ISSUE_JSON" | jq -r .body)
    
    echo "## Issue #$ISSUE_ID: $TITLE" >> "$PROMPT_FILE"
    echo "$BODY" >> "$PROMPT_FILE"
    echo "" >> "$PROMPT_FILE"
    
    # Extract spec paths (naively looking for docs/specs/*.md in the body)
    # This matches lines like "- `docs/specs/auth.md`"
    SPECS=$(echo "$BODY" | grep -oE "docs/specs/[a-zA-Z0-9_-]+\.md" || true)
    for SPEC in $SPECS; do
        SPECS_TO_READ["$SPEC"]=1
    done
done

echo -e "\n# Specifications" >> "$PROMPT_FILE"
for SPEC in "${!SPECS_TO_READ[@]}"; do
    if [ -f "$SPEC" ]; then
        echo "## $SPEC" >> "$PROMPT_FILE"
        cat "$SPEC" >> "$PROMPT_FILE"
        echo "" >> "$PROMPT_FILE"
    else
        echo "Warning: Spec $SPEC referenced but not found on disk."
    fi
done

# 4. Spawning the agent (assuming `pi` or similar CLI)
echo "Context assembled in $PROMPT_FILE. Spawning agent..."
rm -f ralph.done ralph.escalate

# Note: In a real environment, you'd invoke your agent CLI here.
# For `pi`, it might be: pi --file "$PROMPT_FILE"
# Here we'll just echo the instruction.
echo ">> Run: pi --file $PROMPT_FILE"
echo ">> (Waiting for ralph.done or ralph.escalate...)"

# 5. Monitoring loop
while true; do
    if [ -f "ralph.done" ]; then
        echo "Agent signaled completion (ralph.done)."
        break
    fi
    if [ -f "ralph.escalate" ]; then
        echo "Agent escalated:"
        cat ralph.escalate
        exit 1
    fi
    sleep 5
done

# 6. Push and Create PR
echo "Pushing branch..."
git push -u origin "$BRANCH_NAME"

PR_BODY="Unified PR for Ralph issues.\n\n"
for ISSUE_ID in "${ISSUES[@]}"; do
    PR_BODY+="Closes #$ISSUE_ID\n"
done

echo "Creating Draft PR..."
gh pr create --title "Ralph: Unified implementation for ${ISSUES[*]}" --body "$PR_BODY" --draft

# 7. CI Monitoring
echo "Monitoring CI..."
MAX_RETRIES=3
RETRIES=0

while [ $RETRIES -lt $MAX_RETRIES ]; do
    # Wait a bit for CI to start
    sleep 10
    
    # Check CI status (gh pr checks exits 0 if all pass, non-zero if pending/fail)
    # We need to distinguish pending vs fail. 
    # gh pr checks --watch waits until completed.
    if gh pr checks --watch; then
        echo "CI Passed! PR is ready for review."
        exit 0
    else
        echo "CI Failed! Preparing retry $RETRIES of $MAX_RETRIES..."
        
        # In a real V1, you'd fetch the CI failure log:
        # gh run list --branch "$BRANCH_NAME" --limit 1 ...
        # And feed it back into the agent session.
        # e.g. pi "CI failed with: $(gh run view --log)"
        
        rm -f ralph.done ralph.escalate
        echo ">> Agent running retry..."
        
        while true; do
            if [ -f "ralph.done" ]; then break; fi
            if [ -f "ralph.escalate" ]; then exit 1; fi
            sleep 5
        done
        
        git push -f origin "$BRANCH_NAME"
        ((RETRIES++))
    fi
done

echo "Unrecoverable CI failure after $MAX_RETRIES retries."
exit 1
