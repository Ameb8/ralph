package prompt_test

import (
	"strings"
	"testing"

	"ralph/internal/github"
	"ralph/internal/prompt"
)

func TestPromptEngine(t *testing.T) {
	engine, err := prompt.New()
	if err != nil {
		t.Fatalf("failed to initialize prompt engine: %v", err)
	}

	// 1. Test BuildIssuePrompt
	issue := &github.Issue{
		Number: 42,
		Title:  "Implement JWT Auth",
		Body:   "## Summary\nAdd JWT authentication middleware.",
	}
	specs := map[string]string{
		"docs/specs/auth.md": "# Auth Spec\nJWT tokens must expire in 1 hour.",
	}
	handoff := "- Previously created user model in db/schema.go"

	promptA, err := engine.BuildIssuePrompt(issue, specs, handoff)
	if err != nil {
		t.Fatalf("failed to build issue prompt: %v", err)
	}

	mustContain(t, promptA, "Implement Issue #42: Implement JWT Auth")
	mustContain(t, promptA, "Add JWT authentication middleware.")
	mustContain(t, promptA, "docs/specs/auth.md")
	mustContain(t, promptA, "JWT tokens must expire in 1 hour.")
	mustContain(t, promptA, "Previously created user model in db/schema.go")
	mustContain(t, promptA, ".ralph-handoff.md")
	mustContain(t, promptA, "ralph.done")
	mustContain(t, promptA, "ralph.escalate")

	// 2. Test BuildDraftPRPrompt
	issues := []*github.Issue{
		issue,
		{
			Number: 45,
			Title:  "Add Refresh Token Endpoint",
			Body:   "## Summary\nAdd /refresh endpoint.",
		},
	}
	finalHandoff := "- Implemented JWT auth (#42)\n- Added refresh endpoint (#45)"

	promptB, err := engine.BuildDraftPRPrompt(issues, finalHandoff)
	if err != nil {
		t.Fatalf("failed to build draft PR prompt: %v", err)
	}

	mustContain(t, promptB, "Task: Draft Unified Pull Request")
	mustContain(t, promptB, "Issue #42: Implement JWT Auth")
	mustContain(t, promptB, "Issue #45: Add Refresh Token Endpoint")
	mustContain(t, promptB, ".ralph/draft-pr.md")
	mustContain(t, promptB, "Closes #<issue_number>")
	mustContain(t, promptB, "The \"How\"")
	mustContain(t, promptB, "git push -u origin HEAD")
	mustContain(t, promptB, "ralph.done")

	// 3. Test BuildCIFixPrompt
	ciLogs := "--- FAIL: TestJWTAuth (0.05s)\n    auth_test.go:42: token expired unexpectedly"
	promptC, err := engine.BuildCIFixPrompt(ciLogs, finalHandoff)
	if err != nil {
		t.Fatalf("failed to build CI fix prompt: %v", err)
	}

	mustContain(t, promptC, "Task: Fix CI Failure")
	mustContain(t, promptC, "token expired unexpectedly")
	mustContain(t, promptC, "Implemented JWT auth (#42)")
	mustContain(t, promptC, "ralph.done")
	mustContain(t, promptC, "git push")
}

func mustContain(t *testing.T, content, substr string) {
	t.Helper()
	if !strings.Contains(content, substr) {
		t.Errorf("expected content to contain %q, but it did not", substr)
	}
}
