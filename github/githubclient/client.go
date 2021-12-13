package githubclient

import (
	"bytes"
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/rs/zerolog/log"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
	"mvdan.cc/xurls"
)

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

	$ gh auth login

Alternatively, configure a token manually in ~/.config/hub:

	github.com:
	- user: <your username>
	  oauth_token: <your token>
	  protocol: https

This configuration file is shared with GitHub's "hub" CLI (https://hub.github.com/),
so if you already use that, spr will automatically pick up your token. 
`

func NewGitHubClient(ctx context.Context, config *config.Config) *client {
	token := findToken(config.Repo.GitHubHost)
	if token == "" {
		fmt.Printf(tokenHelpText, config.Repo.GitHubHost)
		os.Exit(3)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	var api *githubv4.Client
	if strings.HasSuffix(config.Repo.GitHubHost, "github.com") {
		api = githubv4.NewClient(tc)
	} else {
		var scheme, host string
		gitHubRemoteUrl, err := url.Parse(config.Repo.GitHubHost)
		check(err)
		if gitHubRemoteUrl.Host == "" {
			host = config.Repo.GitHubHost
			scheme = "https"
		} else {
			host = gitHubRemoteUrl.Host
			scheme = gitHubRemoteUrl.Scheme
		}
		api = githubv4.NewEnterpriseClient(fmt.Sprintf("%s://%s/api/graphql", scheme, host), tc)
	}
	return &client{
		config: config,
		api:    api,
	}
}

type client struct {
	config *config.Config
	api    *githubv4.Client
}

var BranchNameRegex = regexp.MustCompile(`pr/[a-zA-Z0-9_\-]+/([a-zA-Z0-9_\-/]+)/([a-f0-9]{8})$`)

func (c *client) GetInfo(ctx context.Context, gitcmd git.GitInterface) *github.GitHubInfo {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github fetch pull requests\n")
	}
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
								MessageHeadline   string
								MessageBody       string
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
		"repo_owner": githubv4.String(c.config.Repo.GitHubRepoOwner),
		"repo_name":  githubv4.String(c.config.Repo.GitHubRepoName),
	}
	err := c.api.Query(ctx, &query, variables)
	check(err)

	branchname := getLocalBranchName(gitcmd)

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

		matches := BranchNameRegex.FindStringSubmatch(node.HeadRefName)
		if matches != nil && matches[1] == branchname {
			pullRequest.Commit = git.Commit{
				CommitID:   matches[2],
				CommitHash: node.Commits.Nodes[0].Commit.Oid,
				Subject:    node.Commits.Nodes[0].Commit.MessageHeadline,
				Body:       node.Commits.Nodes[0].Commit.MessageBody,
			}

			checkStatus := github.CheckStatusFail
			switch node.Commits.Nodes[0].Commit.StatusCheckRollup.State {
			case "SUCCESS":
				checkStatus = github.CheckStatusPass
			case "PENDING":
				checkStatus = github.CheckStatusPending
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

	info := &github.GitHubInfo{
		UserName:     query.Viewer.Login,
		RepositoryID: query.Repository.ID,
		LocalBranch:  branchname,
		PullRequests: requests,
	}

	log.Debug().Interface("Info", info).Msg("GetInfo")

	return info
}

func (c *client) CreatePullRequest(ctx context.Context,
	info *github.GitHubInfo, commit git.Commit, prevCommit *git.Commit) *github.PullRequest {

	baseRefName := c.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}
	headRefName := branchNameFromCommit(info, commit)

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", headRefName).Str("ToBranch", baseRefName).
		Msg("CreatePullRequest")

	var mutation struct {
		CreatePullRequest struct {
			PullRequest struct {
				ID     string
				Number int
			}
		} `graphql:"createPullRequest(input: $input)"`
	}
	commitBody := githubv4.String(formatBody(commit, info.PullRequests))
	input := githubv4.CreatePullRequestInput{
		RepositoryID: info.RepositoryID,
		BaseRefName:  githubv4.String(baseRefName),
		HeadRefName:  githubv4.String(headRefName),
		Title:        githubv4.String(commit.Subject),
		Body:         &commitBody,
	}
	err := c.api.Mutate(ctx, &mutation, input, nil)
	check(err)

	pr := &github.PullRequest{
		ID:         mutation.CreatePullRequest.PullRequest.ID,
		Number:     mutation.CreatePullRequest.PullRequest.Number,
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

func formatStackMarkdown(commit git.Commit, stack []*github.PullRequest) string {
	var buf bytes.Buffer
	for i := len(stack) - 1; i >= 0; i-- {
		isCurrent := stack[i].Commit == commit
		var suffix string
		if isCurrent {
			suffix = " ⮜"
		} else {
			suffix = ""
		}
		buf.WriteString(fmt.Sprintf("- #%d%s\n", stack[i].Number, suffix))
	}

	return buf.String()
}

var issueReferenceRegex = regexp.MustCompile(`(?mi)((close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved)\s)?([a-zA-Z-]+\/[a-zA-Z-]+#\d+|#\d+|https?://.+?/[a-zA-Z-]+\/[a-zA-Z-]+/(issues|pull)/\d+)`)

// linkifyPlainLinks replaces plain links in the body by <a> tags, which makes them clickable
// inside a GitHub code area.
func linkifyPlainLinks(body string) string {
	return string(xurls.Relaxed.ReplaceAll([]byte(body), []byte("<a href=\"$1\">$1</a>")))
}

func wrapInMarkdown(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}

	// Extract issue references for GitHub to find them outside the code block
	refs := issueReferenceRegex.FindAllString(s, -1)
	var trailer bytes.Buffer
	if len(refs) > 0 {
		trailer.WriteString("**Issue references**: \n")
		for _, ref := range refs {
			trailer.WriteString(fmt.Sprintf("\n - %s", ref))
		}
	}

	return fmt.Sprintf("<pre>\n%s\n</pre>\n%s", linkifyPlainLinks(s), trailer.String())
}

func formatBody(commit git.Commit, stack []*github.PullRequest) string {
	if len(stack) <= 1 {
		return wrapInMarkdown(commit.Body)
	}

	return fmt.Sprintf("**Stack**:\n%s\n\n%s",
		formatStackMarkdown(commit, stack),
		addManualMergeNotice(wrapInMarkdown(commit.Body)))
}

func addManualMergeNotice(body string) string {
	return body + "\n\n" +
		"⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). " +
		"Do not merge manually using the UI - doing so may have unexpected results.*"
}

func (c *client) UpdatePullRequest(ctx context.Context,
	info *github.GitHubInfo, pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github update %d - %s\n", pr.Number, pr.Title)
	}

	baseRefName := c.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("PR", pr).Msg("UpdatePullRequest")

	var mutation struct {
		UpdatePullRequest struct {
			PullRequest struct {
				Number int
			}
		} `graphql:"updatePullRequest(input: $input)"`
	}
	baseRefNameStr := githubv4.String(baseRefName)
	subject := githubv4.String(commit.Subject)
	body := githubv4.String(formatBody(commit, info.PullRequests))
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

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github add comment %d: %s\n", pr.Number, pr.Title)
	}
}

func (c *client) MergePullRequest(ctx context.Context, pr *github.PullRequest) {
	log.Debug().Interface("PR", pr).Msg("MergePullRequest")

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

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github merge %d: %s\n", pr.Number, pr.Title)
	}
}

func (c *client) ClosePullRequest(ctx context.Context, pr *github.PullRequest) {
	log.Debug().Interface("PR", pr).Msg("ClosePullRequest")
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
