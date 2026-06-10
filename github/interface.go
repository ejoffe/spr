package github

import (
	"context"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
)

type GitHubInterface interface {
	// GetInfo returns the list of pull requests from GitHub which match the local stack of commits
	GetInfo(ctx context.Context, gitcmd git.GitInterface) *GitHubInfo

	// GetAssignableUsers returns a list of valid GitHub users that can review the pull request
	GetAssignableUsers(ctx context.Context) []RepoAssignee

	// CreatePullRequest creates a pull request
	CreatePullRequest(ctx context.Context, gitcmd git.GitInterface, info *GitHubInfo, commit git.Commit, prevCommit *git.Commit) *PullRequest

	// UpdatePullRequest updates a pull request with current commit
	UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface, info *GitHubInfo, pullRequests []*PullRequest, pr *PullRequest, commit git.Commit, prevCommit *git.Commit)

	// AddReviewers adds a reviewer to the given pull request
	AddReviewers(ctx context.Context, pr *PullRequest, userIDs []string)

	// CommentPullRequest add a comment to the given pull request
	CommentPullRequest(ctx context.Context, pr *PullRequest, comment string)

	// MergePullRequest merged the given pull request
	MergePullRequest(ctx context.Context, pr *PullRequest, mergeMethod genclient.PullRequestMergeMethod)

	// ClosePullRequest closes the given pull request
	ClosePullRequest(ctx context.Context, pr *PullRequest)
}

type GitHubInfo struct {
	UserName     string
	RepositoryID string
	// HeadRepositoryID is the GitHub node ID of the repository that owns the
	// PR head branches (the fork). Empty for same-fork PRs, in which case
	// branches live in RepositoryID. When non-empty and different from
	// RepositoryID, spr is operating in cross-fork mode: branches are pushed
	// to the fork and PRs are opened against RepositoryID (the upstream).
	HeadRepositoryID string
	LocalBranch      string
	PullRequests     []*PullRequest
}

type RepoAssignee struct {
	ID    string
	Login string
	Name  string
}

func (i *GitHubInfo) Key() string {
	return i.RepositoryID + "_" + i.LocalBranch
}
