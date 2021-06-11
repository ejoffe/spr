package githubclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/rs/zerolog/log"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func NewGitHubClient(ctx context.Context, config *config.Config) *client {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Printf("GitHub OAuth Token Required\n")
		fmt.Printf("Make one at: https://%s/settings/tokens\n", "github.com")
		fmt.Printf("And set an env variable called GITHUB_TOKEN with it's value\n")
		os.Exit(3)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	api := githubv4.NewClient(tc)
	return &client{
		config: config,
		api:    api,
	}
}

type client struct {
	config *config.Config
	api    *githubv4.Client
}

var pullRequestRegex = regexp.MustCompile(`pr/[a-zA-Z0-9_\-]+/([a-zA-Z0-9_\-/]+)/([a-f0-9]{8})$`)

func (c *client) GetInfo(ctx context.Context) *github.GitHubInfo {
	var query struct {
		Viewer struct {
			Login        string
			PullRequests struct {
				Nodes []struct {
					ID             string
					Number         int
					Title          string
					BaseRefName    string
					HeadRefName    string
					Mergeable      string
					ReviewDecision string
					Repository     struct {
						ID string
					}
					Commits struct {
						Nodes []struct {
							Commit struct {
								Oid               string
								StatusCheckRollup struct {
									State string
								}
							}
						}
					} `graphql:"commits(first:100)"`
				}
			} `graphql:"pullRequests(first:100, states:[OPEN])"`
		}
		Repository struct {
			ID string
		} `graphql:"repository(owner:$repo_owner, name:$repo_name)"`
	}
	variables := map[string]interface{}{
		"repo_owner": githubv4.String(c.config.GitHubRepoOwner),
		"repo_name":  githubv4.String(c.config.GitHubRepoName),
	}
	err := c.api.Query(ctx, &query, variables)
	check(err)

	var branchname string
	mustgit("branch --show-current", &branchname)

	var requests []*github.PullRequest
	for _, node := range query.Viewer.PullRequests.Nodes {
		if query.Repository.ID != node.Repository.ID {
			continue
		}
		pullRequest := &github.PullRequest{
			ID:         node.ID,
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
			}

			checkStatus := github.CheckStatusUnknown
			switch node.Commits.Nodes[0].Commit.StatusCheckRollup.State {

			case "SUCCESS":
				checkStatus = github.CheckStatusPass
			case "PENDING":
				checkStatus = github.CheckStatusPending
			default:
				checkStatus = github.CheckStatusFail
			}

			pullRequest.MergeStatus = github.PullRequestMergeStatus{
				ChecksPass:     checkStatus,
				ReviewApproved: node.ReviewDecision == "APPROVED",
				NoConflicts:    node.Mergeable == "MERGEABLE",
			}

			requests = append(requests, pullRequest)
		}
	}

	requests = github.SortPullRequests(requests, c.config)

	return &github.GitHubInfo{
		UserName:     query.Viewer.Login,
		RepositoryID: query.Repository.ID,
		LocalBranch:  branchname,
		PullRequests: requests,
	}
}

func (c *client) CreatePullRequest(ctx context.Context,
	info *github.GitHubInfo, commit git.Commit, prevCommit *git.Commit) *github.PullRequest {
	log.Debug().Interface("commit", commit).Interface("prev", prevCommit).
		Msg("createGithubPullRequest")

	baseRefName := "master"
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}
	headRefName := branchNameFromCommit(info, commit)

	var mutation struct {
		CreatePullRequest struct {
			PullRequest struct {
				ID     string
				Number int
			}
		} `graphql:"createPullRequest(input: $input)"`
	}
	commitBody := githubv4.String(commit.Body)
	input := githubv4.CreatePullRequestInput{
		RepositoryID: info.RepositoryID,
		BaseRefName:  githubv4.String(baseRefName),
		HeadRefName:  githubv4.String(headRefName),
		Title:        githubv4.String(commit.Subject),
		Body:         &commitBody,
	}
	err := c.api.Mutate(ctx, &mutation, input, nil)
	check(err)

	return &github.PullRequest{
		ID:         mutation.CreatePullRequest.PullRequest.ID,
		Number:     mutation.CreatePullRequest.PullRequest.Number,
		FromBranch: baseRefName,
		ToBranch:   headRefName,
		Commit:     commit,
		Title:      commit.Subject,
		MergeStatus: github.PullRequestMergeStatus{
			ChecksPass:     github.CheckStatusUnknown,
			ReviewApproved: false,
			NoConflicts:    false,
			Stacked:        false,
		},
	}
}

func (c *client) UpdatePullRequest(ctx context.Context,
	info *github.GitHubInfo, pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {
	log.Debug().Interface("commit", commit).Interface("prevCommit", prevCommit).
		Interface("pr", pr).Msg("updateGithubPullRequest")

	baseRefName := "master"
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}

	var mutation struct {
		UpdatePullRequest struct {
			PullRequest struct {
				Number int
			}
		} `graphql:"updatePullRequest(input: $input)"`
	}
	baseRefNameStr := githubv4.String(baseRefName)
	subject := githubv4.String(commit.Subject)
	body := githubv4.String(commit.Body)
	input := githubv4.UpdatePullRequestInput{
		PullRequestID: pr.ID,
		BaseRefName:   &baseRefNameStr,
		Title:         &subject,
		Body:          &body,
	}
	err := c.api.Mutate(ctx, &mutation, input, nil)
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
	var updatepr struct {
		PullRequest struct {
			ClientMutationID string
		} `graphql:"addComment(input: $input)"`
	}
	updatePRInput := githubv4.AddCommentInput{
		SubjectID: pr.ID,
		Body:      githubv4.String(comment),
	}
	err := c.api.Mutate(ctx, &updatepr, updatePRInput, nil)
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request update failed")
	}
}

func (c *client) MergePullRequest(ctx context.Context, pr *github.PullRequest) {
	var mergepr struct {
		MergePullRequest struct {
			PullRequest struct {
				Number int
			}
		} `graphql:"mergePullRequest(input: $input)"`
	}
	mergeMethod := githubv4.PullRequestMergeMethodRebase
	mergePRInput := githubv4.MergePullRequestInput{
		PullRequestID: pr.ID,
		MergeMethod:   &mergeMethod,
	}
	err := c.api.Mutate(ctx, &mergepr, mergePRInput, nil)
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request merge failed")
	}
	check(err)
}

func (c *client) ClosePullRequest(ctx context.Context, pr *github.PullRequest) {
	var closepr struct {
		ClosePullRequest struct {
			PullRequest struct {
				Number int
			}
		} `graphql:"closePullRequest(input: $input)"`
	}
	closePRInput := githubv4.ClosePullRequestInput{
		PullRequestID: pr.ID,
	}
	err := c.api.Mutate(ctx, &closepr, closePRInput, nil)
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request close failed")
	}
}

func branchNameFromCommit(info *github.GitHubInfo, commit git.Commit) string {
	return "pr/" + info.UserName + "/" + info.LocalBranch + "/" + commit.CommitID
}

func mustgit(argStr string, output *string) {
	err := gitcmd(argStr, output)
	check(err)
}

func gitcmd(argStr string, output *string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command
	args := strings.Split(argStr, " ")
	cmd := exec.Command("git", args...)
	envVarsToDerive := []string{
		"SSH_AUTH_SOCK",
		"SSH_AGENT_PID",
		"HOME",
		"XDG_CONFIG_HOME",
	}
	cmd.Env = []string{"EDITOR=/usr/bin/true"}
	for _, env := range envVarsToDerive {
		envval := os.Getenv(env)
		if envval != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env, envval))
		}
	}

	if output != nil {
		out, err := cmd.CombinedOutput()
		*output = strings.TrimSpace(string(out))
		if err != nil {
			return err
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "git error: %s", string(out))
			return err
		}
	}
	return nil
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
