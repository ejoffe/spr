package github

import (
	"context"

	"github.com/ejoffe/spr/git"
	"github.com/shurcooL/githubv4"
)

type GitHubInterface interface {
	GetInfo(ctx context.Context, gitcmd git.GitInterface) *GitHubInfo
	CreatePullRequest(ctx context.Context, info *GitHubInfo, commit git.Commit, prevCommit *git.Commit) *PullRequest
	UpdatePullRequest(ctx context.Context, info *GitHubInfo, pr *PullRequest, commit git.Commit, prevCommit *git.Commit)
	CommentPullRequest(ctx context.Context, pr *PullRequest, comment string)
	MergePullRequest(ctx context.Context, pr *PullRequest, mergeMethod githubv4.PullRequestMergeMethod)
	ClosePullRequest(ctx context.Context, pr *PullRequest)
}

type GitHubInfo struct {
	UserName     string
	RepositoryID string
	LocalBranch  string
	PullRequests []*PullRequest
}
