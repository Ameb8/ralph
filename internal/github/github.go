package github

import (
	"context"
)

type Issue struct {
	Number int
	Title  string
	Body   string
}

type CIStatus string

type GitHubClient interface {
	GetIssue(ctx context.Context, number int) (*Issue, error)
	CreatePR(ctx context.Context, title, bodyFile, baseBranch string) (string, error)
	GetCIStatus(ctx context.Context, branch string) (CIStatus, error)
	GetFailedLogs(ctx context.Context, branch string) (string, error)
}
