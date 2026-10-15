package github

import (
	"context"
)

// Issue represents a GitHub issue.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// CIStatus represents the status of CI checks.
type CIStatus string

const (
	CIStatusSuccess CIStatus = "success"
	CIStatusFailure CIStatus = "failure"
	CIStatusPending CIStatus = "pending"
)

// GitHubClient defines the contract for interacting with GitHub.
type GitHubClient interface {
	GetIssue(ctx context.Context, number int) (*Issue, error)
	CreatePR(ctx context.Context, title, bodyFile, baseBranch string) (string, error)
	GetCIStatus(ctx context.Context, branch string) (CIStatus, error)
	GetFailedLogs(ctx context.Context, branch string) (string, error)
}
