package githubclient

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/gen/github_client"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

func NewGitHubClient(ctx context.Context, config *config.Config) *client {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Printf("GitHub OAuth Token Required\n")
		fmt.Printf("Make one at: https://github.com/settings/tokens\n")
		fmt.Printf("With repo scope selected.\n")
		fmt.Printf("And set an env variable called GITHUB_TOKEN with it's value.\n")
		os.Exit(3)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	api := github_client.NewClient("https://api.github.com/graphql", tc)
	return &client{
		config: config,
		api:    api,
	}
}

type client struct {
	config *config.Config
	api    *github_client.Client
}

var pullRequestRegex = regexp.MustCompile(`pr/[a-zA-Z0-9_\-]+/([a-zA-Z0-9_\-/]+)/([a-f0-9]{8})$`)

func (c *client) GetInfo(ctx context.Context, gitcmd git.GitInterface) *github.GitHubInfo {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github fetch pull requests\n")
	}
	resp, err := c.api.GetInfo(ctx, &github_client.GetInfoInputArgs{
		Repo_owner: c.config.Repo.GitHubRepoOwner,
		Repo_name:  c.config.Repo.GitHubRepoName,
	})
	check(err)

	branchname := getLocalBranchName(gitcmd)

	var requests []*github.PullRequest
	for _, node := range resp.Viewer.PullRequests.Nodes {
		if resp.Repository.Id != node.Repository.Id {
			continue
		}
		pullRequest := &github.PullRequest{
			ID:         node.Id,
			Number:     node.Number,
			Title:      node.Title,
			FromBranch: node.HeadRefName,
			ToBranch:   node.BaseRefName,
		}

		matches := pullRequestRegex.FindStringSubmatch(node.HeadRefName)
		if matches != nil && matches[1] == branchname {
			pullRequest.Commit = git.Commit{
				CommitID:   matches[2],
				CommitHash: node.Commits.Nodes[0].Commit.Oid,
				Subject:    node.Commits.Nodes[0].Commit.MessageHeadline,
				Body:       node.Commits.Nodes[0].Commit.MessageBody,
			}

			checkStatus := github.CheckStatusFail
			switch node.Commits.Nodes[0].Commit.StatusCheckRollup.State {
			case github_client.StatusState_SUCCESS:
				checkStatus = github.CheckStatusPass
			case github_client.StatusState_PENDING:
				checkStatus = github.CheckStatusPending
			}

			pullRequest.MergeStatus = github.PullRequestMergeStatus{
				ChecksPass:  checkStatus,
				NoConflicts: node.Mergeable == github_client.MergeableState_MERGEABLE,
			}

			if node.ReviewDecision != nil &&
				*node.ReviewDecision == github_client.PullRequestReviewDecision_APPROVED {
				pullRequest.MergeStatus.ReviewApproved = true
			} else {
				pullRequest.MergeStatus.ReviewApproved = false
			}

			requests = append(requests, pullRequest)
		}
	}

	requests = github.SortPullRequests(requests, c.config)

	info := &github.GitHubInfo{
		UserName:     resp.Viewer.Login,
		RepositoryID: resp.Repository.Id,
		LocalBranch:  branchname,
		PullRequests: requests,
	}

	log.Debug().Interface("Info", info).Msg("GetInfo")

	return info
}

func (c *client) CreatePullRequest(ctx context.Context,
	info *github.GitHubInfo, commit git.Commit, prevCommit *git.Commit) *github.PullRequest {

	baseRefName := "master"
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}
	headRefName := branchNameFromCommit(info, commit)

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", headRefName).Str("ToBranch", baseRefName).
		Msg("CreatePullRequest")

	resp, err := c.api.CreatePullRequest(ctx, &github_client.CreatePullRequestInputArgs{
		Input: github_client.CreatePullRequestInput{
			RepositoryId: info.RepositoryID,
			BaseRefName:  baseRefName,
			HeadRefName:  headRefName,
			Title:        commit.Subject,
			Body:         &commit.Body,
		},
	})
	check(err)

	pr := &github.PullRequest{
		ID:         resp.CreatePullRequest.PullRequest.Id,
		Number:     resp.CreatePullRequest.PullRequest.Number,
		FromBranch: headRefName,
		ToBranch:   baseRefName,
		Commit:     commit,
		Title:      commit.Subject,
		MergeStatus: github.PullRequestMergeStatus{
			ChecksPass:     github.CheckStatusUnknown,
			ReviewApproved: false,
			NoConflicts:    false,
			Stacked:        false,
		},
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github create %d: %s\n", pr.Number, pr.Title)
	}

	return pr
}

func (c *client) UpdatePullRequest(ctx context.Context,
	info *github.GitHubInfo, pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github update %d - %s\n", pr.Number, pr.Title)
	}

	baseRefName := "master"
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("PR", pr).Msg("UpdatePullRequest")

	_, err := c.api.UpdatePullRequest(ctx, &github_client.UpdatePullRequestInputArgs{
		Input: github_client.UpdatePullRequestInput{
			PullRequestId: pr.ID,
			BaseRefName:   &baseRefName,
			Title:         &commit.Subject,
			Body:          &commit.Body,
		},
	})
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request update failed")
	}
}

func (c *client) CommentPullRequest(ctx context.Context, pr *github.PullRequest, comment string) {
	_, err := c.api.CommentPullRequest(ctx, &github_client.CommentPullRequestInputArgs{
		Input: github_client.AddCommentInput{
			SubjectId: pr.ID,
			Body:      comment,
		},
	})
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request update failed")
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github add comment %d: %s\n", pr.Number, pr.Title)
	}
}

func (c *client) MergePullRequest(ctx context.Context, pr *github.PullRequest) {
	log.Debug().Interface("PR", pr).Msg("MergePullRequest")

	mergeMethod := github_client.PullRequestMergeMethod_REBASE
	_, err := c.api.MergePullRequest(ctx, &github_client.MergePullRequestInputArgs{
		Input: github_client.MergePullRequestInput{
			PullRequestId: pr.ID,
			MergeMethod:   &mergeMethod,
		},
	})
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request merge failed")
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github merge %d: %s\n", pr.Number, pr.Title)
	}
}

func (c *client) ClosePullRequest(ctx context.Context, pr *github.PullRequest) {
	log.Debug().Interface("PR", pr).Msg("ClosePullRequest")
	_, err := c.api.ClosePullRequest(ctx, &github_client.ClosePullRequestInputArgs{
		Input: github_client.ClosePullRequestInput{
			PullRequestId: pr.ID,
		},
	})
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request close failed")
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github close %d: %s\n", pr.Number, pr.Title)
	}
}

func getLocalBranchName(gitcmd git.GitInterface) string {
	var output string
	err := gitcmd.Git("branch", &output)
	check(err)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "* ") {
			return line[2:]
		}
	}
	panic("cannot determine local git branch name")
}

func branchNameFromCommit(info *github.GitHubInfo, commit git.Commit) string {
	return "pr/" + info.UserName + "/" + info.LocalBranch + "/" + commit.CommitID
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
