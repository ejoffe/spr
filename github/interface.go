package github

import (
	"context"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
)

type GitHubInterface interface {
	GetInfo(ctx context.Context, gitcmd git.GitInterface) *GitHubInfo
	GetAssignableUsers(ctx context.Context) []RepoAssignee
	CreatePullRequest(ctx context.Context, info *GitHubInfo, commit git.Commit, prevCommit *git.Commit) *PullRequest
	UpdatePullRequest(ctx context.Context, info *GitHubInfo, pr *PullRequest, commit git.Commit, prevCommit *git.Commit)
	AddReviewers(ctx context.Context, pr *PullRequest, userIDs []string)
	CommentPullRequest(ctx context.Context, pr *PullRequest, comment string)
	MergePullRequest(ctx context.Context, pr *PullRequest, mergeMethod genclient.PullRequestMergeMethod)
	ClosePullRequest(ctx context.Context, pr *PullRequest)
}

type GitHubInfo struct {
	UserName     string
	RepositoryID string
	LocalBranch  string
	PullRequests []*PullRequest
}

type RepoAssignee struct {
	ID    string
	Login string
	Name  string
}
