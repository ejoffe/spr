package forge

import (
	"context"
	"fmt"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

type ForgeInterface interface {
	// GetInfo returns the list of pull requests from the forge which match the local stack of commits
	GetInfo(ctx context.Context, gitcmd git.GitInterface) *ForgeInfo

	// GetAssignableUsers returns a list of valid users that can review the pull request
	GetAssignableUsers(ctx context.Context) []RepoAssignee

	// CreatePullRequest creates a pull request
	CreatePullRequest(ctx context.Context, gitcmd git.GitInterface, info *ForgeInfo, commit git.Commit, prevCommit *git.Commit) *PullRequest

	// UpdatePullRequest updates a pull request with current commit
	UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface, info *ForgeInfo, pullRequests []*PullRequest, pr *PullRequest, commit git.Commit, prevCommit *git.Commit)

	// AddReviewers adds a reviewer to the given pull request
	AddReviewers(ctx context.Context, pr *PullRequest, userIDs []string)

	// CommentPullRequest add a comment to the given pull request
	CommentPullRequest(ctx context.Context, pr *PullRequest, comment string)

	// MergePullRequest merged the given pull request
	MergePullRequest(ctx context.Context, pr *PullRequest, mergeMethod config.MergeMethod)

	// ClosePullRequest closes the given pull request
	ClosePullRequest(ctx context.Context, pr *PullRequest)
}

type ForgeInfo struct {
	UserName       string
	RepositoryID   string
	LocalBranch    string
	PullRequests   []*PullRequest
	PRNumberPrefix string // Used to format PR bodies with the right auto-linking format
}

type RepoAssignee struct {
	ID    string
	Login string
	Name  string
}

func (i *ForgeInfo) Key() string {
	return i.RepositoryID + "_" + i.LocalBranch
}

// BuildPullRequestStack takes a pre-built map of commitID → PullRequest and assembles
// them into an ordered stack by walking the ToBranch chain from the top PR down to
// the targetBranch. Both GitHub and GitLab clients build their pullRequestMap from
// forge-specific API responses, then call this shared function.
func BuildPullRequestStack(
	targetBranch string,
	localCommitStack []git.Commit,
	pullRequestMap map[string]*PullRequest,
) []*PullRequest {
	if len(localCommitStack) == 0 || len(pullRequestMap) == 0 {
		return []*PullRequest{}
	}

	// find top pr by walking local commits from newest to oldest
	var currpr *PullRequest
	for i := len(localCommitStack) - 1; i >= 0; i-- {
		if pr, found := pullRequestMap[localCommitStack[i].CommitID]; found {
			currpr = pr
			break
		}
	}

	// The list of commits from the command line actually starts at the
	//  most recent commit. In order to reverse the list we use a
	//  custom prepend function instead of append.
	prepend := func(l []*PullRequest, pr *PullRequest) []*PullRequest {
		l = append(l, &PullRequest{})
		copy(l[1:], l)
		l[0] = pr
		return l
	}

	// build pr stack by walking ToBranch chain
	var pullRequests []*PullRequest
	for currpr != nil {
		pullRequests = prepend(pullRequests, currpr)
		if currpr.ToBranch == targetBranch {
			break
		}

		matches := git.BranchNameRegex.FindStringSubmatch(currpr.ToBranch)
		if matches == nil {
			panic(fmt.Errorf("invalid base branch for pull request: %s", currpr.ToBranch))
		}
		nextCommitID := matches[2]
		currpr = pullRequestMap[nextCommitID]
	}

	return pullRequests
}
