package prompt

import (
	"ralph/internal/github"
)

type Assembler interface {
	BuildIssuePrompt(issue *github.Issue, specs map[string]string, handoff string) (string, error)
	BuildDraftPRPrompt(issues []*github.Issue, handoff string) (string, error)
	BuildCIFixPrompt(logs string, handoff string) (string, error)
}
