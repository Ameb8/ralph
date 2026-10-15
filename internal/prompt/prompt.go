package prompt

import (
	"bytes"
	"fmt"
	"text/template"

	"ralph/internal/github"
	"ralph/templates"
)

// Assembler defines the contract for prompt construction.
type Assembler interface {
	BuildIssuePrompt(issue *github.Issue, specs map[string]string, handoff string) (string, error)
	BuildDraftPRPrompt(issues []*github.Issue, handoff string) (string, error)
	BuildCIFixPrompt(logs string, handoff string) (string, error)
}

// Engine implements Assembler using text/template and embedded template files.
type Engine struct {
	tmpl *template.Template
}

var _ Assembler = (*Engine)(nil)

// New creates a new prompt Engine with embedded templates parsed.
func New() (*Engine, error) {
	t, err := template.ParseFS(templates.FS, "*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt templates: %w", err)
	}
	return &Engine{tmpl: t}, nil
}

type IssueData struct {
	Issue   *github.Issue
	Specs   map[string]string
	Handoff string
}

type DraftPRData struct {
	Issues  []*github.Issue
	Handoff string
}

type CIFixData struct {
	Logs    string
	Handoff string
}

// BuildIssuePrompt constructs the prompt for Phase A issue implementation.
func (e *Engine) BuildIssuePrompt(issue *github.Issue, specs map[string]string, handoff string) (string, error) {
	data := IssueData{
		Issue:   issue,
		Specs:   specs,
		Handoff: handoff,
	}
	var buf bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&buf, "issue.tmpl", data); err != nil {
		return "", fmt.Errorf("failed to execute issue template: %w", err)
	}
	return buf.String(), nil
}

// BuildDraftPRPrompt constructs the prompt for Phase B PR drafting.
func (e *Engine) BuildDraftPRPrompt(issues []*github.Issue, handoff string) (string, error) {
	data := DraftPRData{
		Issues:  issues,
		Handoff: handoff,
	}
	var buf bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&buf, "draft_pr.tmpl", data); err != nil {
		return "", fmt.Errorf("failed to execute draft_pr template: %w", err)
	}
	return buf.String(), nil
}

// BuildCIFixPrompt constructs the prompt for Phase C CI failure resolution.
func (e *Engine) BuildCIFixPrompt(logs string, handoff string) (string, error) {
	data := CIFixData{
		Logs:    logs,
		Handoff: handoff,
	}
	var buf bytes.Buffer
	if err := e.tmpl.ExecuteTemplate(&buf, "ci_fix.tmpl", data); err != nil {
		return "", fmt.Errorf("failed to execute ci_fix template: %w", err)
	}
	return buf.String(), nil
}
