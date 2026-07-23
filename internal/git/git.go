package git

import (
	"context"
)

type GitClient interface {
	CreateAndCheckoutBranch(ctx context.Context, branchName string) error
	Push(ctx context.Context, branchName string, force bool) error
	GetDiff(ctx context.Context, baseBranch string) (string, error)
}
