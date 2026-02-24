package githubclient

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/forge"
	"github.com/ejoffe/spr/forge/template/config_fetcher"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/githubclient/fezzik_types"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

//go:generate go run github.com/inigolabs/fezzik --config fezzik.yaml

// hub cli config (https://hub.github.com)
type hubCLIConfig map[string][]struct {
	User       string `yaml:"user"`
	OauthToken string `yaml:"oauth_token"`
	Protocol   string `yaml:"protocol"`
}

// readHubCLIConfig finds and deserialized the config file for
// Github's "hub" CLI (https://hub.github.com/).
func readHubCLIConfig() (hubCLIConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	f, err := os.Open(path.Join(homeDir, ".config", "hub"))
	if err != nil {
		return nil, fmt.Errorf("failed to open hub config file: %w", err)
	}

	var cfg hubCLIConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse hub config file: %w", err)
	}

	return cfg, nil
}

// gh cli config (https://cli.github.com)
type ghCLIConfig map[string]struct {
	User        string `yaml:"user"`
	OauthToken  string `yaml:"oauth_token"`
	GitProtocol string `yaml:"git_protocol"`
}

func readGhCLIConfig() (*ghCLIConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	f, err := os.Open(path.Join(homeDir, ".config", "gh", "hosts.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to open gh cli config file: %w", err)
	}

	var cfg ghCLIConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse hub config file: %w", err)
	}

	return &cfg, nil
}

func findToken(githubHost string) string {
	// Try environment variable first
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token
	}

	// Try ~/.config/gh/hosts.yml
	cfg, err := readGhCLIConfig()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read gh cli config file")
	} else {
		for host, user := range *cfg {
			if host == githubHost {
				return user.OauthToken
			}
		}
	}

	// Try ~/.config/hub
	hubCfg, err := readHubCLIConfig()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read hub config file")
		return ""
	}

	if c, ok := hubCfg["github.com"]; ok {
		if len(c) == 0 {
			log.Warn().Msg("no token found in hub config file")
			return ""
		}
		if len(c) > 1 {
			log.Warn().Msgf("multiple tokens found in hub config file, using first one: %s", c[0].User)
		}

		return c[0].OauthToken
	}

	return ""
}

const tokenHelpText = `
No GitHub OAuth token found! You can either create one
at https://%s/settings/tokens and set the GITHUB_TOKEN environment variable,
or use the official "gh" CLI (https://cli.github.com) config to log in:

	$ gh auth login --insecure-storage

Alternatively, configure a token manually in ~/.config/hub:

	github.com:
	- user: <your username>
	  oauth_token: <your token>
	  protocol: https

This configuration file is shared with GitHub's "hub" CLI (https://hub.github.com/),
so if you already use that, spr will automatically pick up your token.
`

func NewGitHubClient(ctx context.Context, config *config.Config) *client {
	token := findToken(config.Repo.ForgeHost)
	if token == "" {
		fmt.Printf(tokenHelpText, config.Repo.ForgeHost)
		os.Exit(3)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	var api genclient.Client
	if strings.HasSuffix(config.Repo.ForgeHost, "github.com") {
		api = genclient.NewClient("https://api.github.com/graphql", tc)
	} else {
		var scheme, host string
		gitHubRemoteUrl, err := url.Parse(config.Repo.ForgeHost)
		check(err)
		if gitHubRemoteUrl.Host == "" {
			host = config.Repo.ForgeHost
			scheme = "https"
		} else {
			host = gitHubRemoteUrl.Host
			scheme = gitHubRemoteUrl.Scheme
		}
		api = genclient.NewClient(fmt.Sprintf("%s://%s/api/graphql", scheme, host), tc)
	}
	return &client{
		config: config,
		api:    api,
	}
}

type client struct {
	config *config.Config
	api    genclient.Client
}

func (c *client) GetInfo(ctx context.Context, gitcmd git.GitInterface) *forge.ForgeInfo {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github fetch pull requests\n")
	}

	var pullRequestConnection fezzik_types.PullRequestConnection
	var loginName string
	var repoID string
	if c.config.Repo.MergeQueue {
		resp, err := c.api.PullRequestsWithMergeQueue(ctx,
			c.config.Repo.RepoOwner,
			c.config.Repo.RepoName)
		check(err)
		pullRequestConnection = resp.Viewer.PullRequests
		loginName = resp.Viewer.Login
		repoID = resp.Repository.Id
	} else {
		resp, err := c.api.PullRequests(ctx,
			c.config.Repo.RepoOwner,
			c.config.Repo.RepoName)
		check(err)
		pullRequestConnection = resp.Viewer.PullRequests
		loginName = resp.Viewer.Login
		repoID = resp.Repository.Id
	}

	targetBranch := c.config.Repo.Branch
	localCommitStack := git.GetLocalCommitStack(c.config, gitcmd)

	pullRequests := matchPullRequestStack(c.config.Repo, targetBranch, localCommitStack, pullRequestConnection)
	for _, pr := range pullRequests {
		if pr.Ready(c.config) {
			pr.MergeStatus.Stacked = true
		} else {
			break
		}
	}

	info := &forge.ForgeInfo{
		UserName:       loginName,
		RepositoryID:   repoID,
		LocalBranch:    git.GetLocalBranchName(gitcmd),
		PullRequests:   pullRequests,
		PRNumberPrefix: "#",
	}

	log.Debug().Interface("Info", info).Msg("GetInfo")
	return info
}

func matchPullRequestStack(
	repoConfig *config.RepoConfig,
	targetBranch string,
	localCommitStack []git.Commit,
	allPullRequests fezzik_types.PullRequestConnection) []*forge.PullRequest {

	if len(localCommitStack) == 0 || allPullRequests.Nodes == nil {
		return []*forge.PullRequest{}
	}

	// pullRequestMap is a map from commit-id to pull request
	pullRequestMap := make(map[string]*forge.PullRequest)
	for _, node := range *allPullRequests.Nodes {
		var commits []git.Commit
		for _, v := range *node.Commits.Nodes {
			for _, line := range strings.Split(v.Commit.MessageBody, "\n") {
				if strings.HasPrefix(line, "commit-id:") {
					commits = append(commits, git.Commit{
						CommitID:   strings.Split(line, ":")[1],
						CommitHash: v.Commit.Oid,
						Subject:    v.Commit.MessageHeadline,
						Body:       v.Commit.MessageBody,
					})
				}
			}
		}

		pullRequest := &forge.PullRequest{
			ID:         node.Id,
			Number:     node.Number,
			Title:      node.Title,
			Body:       node.Body,
			FromBranch: node.HeadRefName,
			ToBranch:   node.BaseRefName,
			Commits:    commits,
			InQueue:    node.MergeQueueEntry != nil,
		}

		matches := git.BranchNameRegex.FindStringSubmatch(node.HeadRefName)
		if matches != nil {
			commit := (*node.Commits.Nodes)[len(*node.Commits.Nodes)-1].Commit
			pullRequest.Commit = git.Commit{
				CommitID:   matches[2],
				CommitHash: commit.Oid,
				Subject:    commit.MessageHeadline,
				Body:       commit.MessageBody,
			}

			checkStatus := forge.CheckStatusPass
			if commit.StatusCheckRollup != nil {
				switch commit.StatusCheckRollup.State {
				case "SUCCESS":
					checkStatus = forge.CheckStatusPass
				case "PENDING":
					checkStatus = forge.CheckStatusPending
				default:
					checkStatus = forge.CheckStatusFail
				}
			}

			pullRequest.MergeStatus = forge.PullRequestMergeStatus{
				ChecksPass:     checkStatus,
				ReviewApproved: node.ReviewDecision != nil && *node.ReviewDecision == "APPROVED",
				NoConflicts:    node.Mergeable == "MERGEABLE",
			}

			pullRequestMap[pullRequest.Commit.CommitID] = pullRequest
		}
	}

	return forge.BuildPullRequestStack(targetBranch, localCommitStack, pullRequestMap)
}

// GetAssignableUsers is taken from github.com/cli/cli/api and is the approach used by the official gh
// client to resolve user IDs to "ID" values for the update PR API calls. See api.RepoAssignableUsers.
func (c *client) GetAssignableUsers(ctx context.Context) []forge.RepoAssignee {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github get assignable users\n")
	}

	users := []forge.RepoAssignee{}
	var endCursor *string
	for {
		resp, err := c.api.AssignableUsers(ctx,
			c.config.Repo.RepoOwner,
			c.config.Repo.RepoName, endCursor)
		if err != nil {
			log.Fatal().Err(err).Msg("get assignable users failed")
			return nil
		}

		for _, node := range *resp.Repository.AssignableUsers.Nodes {
			user := forge.RepoAssignee{
				ID:    node.Id,
				Login: node.Login,
			}
			if node.Name != nil {
				user.Name = *node.Name
			}
			users = append(users, user)
		}
		if !resp.Repository.AssignableUsers.PageInfo.HasNextPage {
			break
		}
		endCursor = resp.Repository.AssignableUsers.PageInfo.EndCursor
	}

	return users
}

func (c *client) CreatePullRequest(ctx context.Context, gitcmd git.GitInterface,
	info *forge.ForgeInfo, commit git.Commit, prevCommit *git.Commit) *forge.PullRequest {

	baseRefName := c.config.Repo.Branch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(c.config, *prevCommit)
	}
	headRefName := git.BranchNameFromCommit(c.config, commit)

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", headRefName).Str("ToBranch", baseRefName).
		Msg("CreatePullRequest")

	templatizer := config_fetcher.PRTemplatizer(c.config, gitcmd)

	body := templatizer.Body(info, commit, nil)
	resp, err := c.api.CreatePullRequest(ctx, genclient.CreatePullRequestInput{
		RepositoryId: info.RepositoryID,
		BaseRefName:  baseRefName,
		HeadRefName:  headRefName,
		Title:        templatizer.Title(info, commit),
		Body:         &body,
		Draft:        &c.config.User.CreateDraftPRs,
	})
	check(err)

	pr := &forge.PullRequest{
		ID:         resp.CreatePullRequest.PullRequest.Id,
		Number:     resp.CreatePullRequest.PullRequest.Number,
		FromBranch: headRefName,
		ToBranch:   baseRefName,
		Commit:     commit,
		Title:      commit.Subject,
		Body:       resp.CreatePullRequest.PullRequest.Body,
		MergeStatus: forge.PullRequestMergeStatus{
			ChecksPass:     forge.CheckStatusUnknown,
			ReviewApproved: false,
			NoConflicts:    false,
			Stacked:        false,
		},
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github create %d : %s\n", pr.Number, pr.Title)
	}

	return pr
}

func (c *client) UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface,
	info *forge.ForgeInfo, pullRequests []*forge.PullRequest, pr *forge.PullRequest,
	commit git.Commit, prevCommit *git.Commit) {

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github update %d : %s\n", pr.Number, pr.Title)
	}

	baseRefName := c.config.Repo.Branch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(c.config, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("PR", pr).Msg("UpdatePullRequest")

	templatizer := config_fetcher.PRTemplatizer(c.config, gitcmd)
	title := templatizer.Title(info, commit)
	body := templatizer.Body(info, commit, pr)
	input := genclient.UpdatePullRequestInput{
		PullRequestId: pr.ID,
		Title:         &title,
		Body:          &body,
	}
	if c.config.User.PreserveTitleAndBody {
		input.Title = nil
		input.Body = nil
	}

	if !pr.InQueue {
		input.BaseRefName = &baseRefName
	}

	_, err := c.api.UpdatePullRequest(ctx, input)

	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request update failed")
	}
}

// AddReviewers adds reviewers to the provided pull request using the requestReviews() API call. It
// takes github user IDs (ID type) as its input. These can be found by first querying the AssignableUsers
// for the repo, and then mapping login name to ID.
func (c *client) AddReviewers(ctx context.Context, pr *forge.PullRequest, userIDs []string) {
	log.Debug().Strs("userIDs", userIDs).Msg("AddReviewers")
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github add reviewers %d : %s - %+v\n", pr.Number, pr.Title, userIDs)
	}
	union := false
	_, err := c.api.AddReviewers(ctx, genclient.RequestReviewsInput{
		PullRequestId: pr.ID,
		Union:         &union,
		UserIds:       &userIDs,
	})
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Strs("userIDs", userIDs).
			Err(err).
			Msg("add reviewers failed")
	}
}

func (c *client) CommentPullRequest(ctx context.Context, pr *forge.PullRequest, comment string) {
	_, err := c.api.CommentPullRequest(ctx, genclient.AddCommentInput{
		SubjectId: pr.ID,
		Body:      comment,
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
		fmt.Printf("> github add comment %d : %s\n", pr.Number, pr.Title)
	}
}

func (c *client) MergePullRequest(ctx context.Context,
	pr *forge.PullRequest, mergeMethod config.MergeMethod) {
	log.Debug().
		Interface("PR", pr).
		Str("mergeMethod", string(mergeMethod)).
		Msg("MergePullRequest")

	apiMergeMethod := toGitHubMergeMethod(mergeMethod)
	var err error
	if c.config.Repo.MergeQueue {
		_, err = c.api.AutoMergePullRequest(ctx, genclient.EnablePullRequestAutoMergeInput{
			PullRequestId: pr.ID,
			MergeMethod:   &apiMergeMethod,
		})
	} else {
		_, err = c.api.MergePullRequest(ctx, genclient.MergePullRequestInput{
			PullRequestId: pr.ID,
			MergeMethod:   &apiMergeMethod,
		})
	}
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("pull request merge failed")
	}
	check(err)

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github merge %d : %s\n", pr.Number, pr.Title)
	}
}

func toGitHubMergeMethod(m config.MergeMethod) genclient.PullRequestMergeMethod {
	switch m {
	case config.MergeMethodMerge:
		return genclient.PullRequestMergeMethod_MERGE
	case config.MergeMethodSquash:
		return genclient.PullRequestMergeMethod_SQUASH
	case config.MergeMethodRebase:
		return genclient.PullRequestMergeMethod_REBASE
	default:
		return genclient.PullRequestMergeMethod_REBASE
	}
}

func (c *client) PullRequestURL(number int) string {
	return fmt.Sprintf("https://%s/%s/%s/pull/%d",
		c.config.Repo.ForgeHost, c.config.Repo.RepoOwner, c.config.Repo.RepoName, number)
}

func (c *client) ClosePullRequest(ctx context.Context, pr *forge.PullRequest) {
	log.Debug().Interface("PR", pr).Msg("ClosePullRequest")
	_, err := c.api.ClosePullRequest(ctx, genclient.ClosePullRequestInput{
		PullRequestId: pr.ID,
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
		fmt.Printf("> github close %d : %s\n", pr.Number, pr.Title)
	}
}

func check(err error) {
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "401 Unauthorized") {
			errmsg := "error : 401 Unauthorized\n"
			errmsg += " make sure GITHUB_TOKEN env variable is set with a valid token\n"
			errmsg += " to create a valid token goto: https://github.com/settings/tokens\n"
			fmt.Fprint(os.Stderr, errmsg)
			os.Exit(-1)
		} else {
			panic(err)
		}
	}
}
