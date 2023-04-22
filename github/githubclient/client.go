package githubclient

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
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

	var api genclient.Client
	if strings.HasSuffix(config.Repo.GitHubHost, "github.com") {
		api = genclient.NewClient("https://api.github.com/graphql", tc)
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

var BranchNameRegex = regexp.MustCompile(`spr/([a-zA-Z0-9_\-/\.]+)/([a-f0-9]{8})$`)

func (c *client) GetInfo(ctx context.Context, gitcmd git.GitInterface) *github.GitHubInfo {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github fetch pull requests\n")
	}
	resp, err := c.api.PullRequests(ctx,
		c.config.Repo.GitHubRepoOwner,
		c.config.Repo.GitHubRepoName)
	check(err)

	branchname := git.GetLocalBranchName(gitcmd)

	var requests []*github.PullRequest
	for _, node := range *resp.Repository.PullRequests.Nodes {
		if resp.Repository.Id != node.Repository.Id {
			continue
		}
		pullRequest := &github.PullRequest{
			ID:         node.Id,
			Number:     node.Number,
			Title:      node.Title,
			Body:       node.Body,
			FromBranch: node.HeadRefName,
			ToBranch:   node.BaseRefName,
		}

		matches := BranchNameRegex.FindStringSubmatch(node.HeadRefName)
		if matches != nil && matches[1] == branchname {
			commit := (*node.Commits.Nodes)[0].Commit
			pullRequest.Commit = git.Commit{
				CommitID:   matches[2],
				CommitHash: commit.Oid,
				Subject:    commit.MessageHeadline,
				Body:       commit.MessageBody,
			}

			checkStatus := github.CheckStatusFail
			if commit.StatusCheckRollup != nil {
				switch commit.StatusCheckRollup.State {
				case "SUCCESS":
					checkStatus = github.CheckStatusPass
				case "PENDING":
					checkStatus = github.CheckStatusPending
				}
			}

			pullRequest.MergeStatus = github.PullRequestMergeStatus{
				ChecksPass:     checkStatus,
				ReviewApproved: node.ReviewDecision != nil && *node.ReviewDecision == "APPROVED",
				NoConflicts:    node.Mergeable == "MERGEABLE",
			}

			requests = append(requests, pullRequest)
		}
	}

	targetBranch := git.GetRemoteBranchName(c.config.Repo, gitcmd)
	requests = sortPullRequests(requests, c.config, targetBranch)

	info := &github.GitHubInfo{
		UserName:     resp.Viewer.Login,
		RepositoryID: resp.Repository.Id,
		LocalBranch:  branchname,
		PullRequests: requests,
	}

	log.Debug().Interface("Info", info).Msg("GetInfo")

	return info
}

// GetAssignableUsers is taken from github.com/cli/cli/api and is the approach used by the official gh
// client to resolve user IDs to "ID" values for the update PR API calls. See api.RepoAssignableUsers.
func (c *client) GetAssignableUsers(ctx context.Context) []github.RepoAssignee {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github get assignable users\n")
	}

	users := []github.RepoAssignee{}
	var endCursor *string
	for {
		resp, err := c.api.AssignableUsers(ctx,
			c.config.Repo.GitHubRepoOwner,
			c.config.Repo.GitHubRepoName, endCursor)
		if err != nil {
			log.Fatal().Err(err).Msg("get assignable users failed")
			return nil
		}

		for _, node := range *resp.Repository.AssignableUsers.Nodes {
			user := github.RepoAssignee{
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
	info *github.GitHubInfo, commit git.Commit, prevCommit *git.Commit) *github.PullRequest {

	baseRefName := git.GetRemoteBranchName(c.config.Repo, gitcmd)
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}
	headRefName := branchNameFromCommit(info, commit)

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", headRefName).Str("ToBranch", baseRefName).
		Msg("CreatePullRequest")

	body := formatBody(commit, info.PullRequests)
	if c.config.Repo.PRTemplatePath != "" {
		pullRequestTemplate, err := readPRTemplate(gitcmd, c.config.Repo.PRTemplatePath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read PR template")
		}
		body, err = insertBodyIntoPRTemplate(body, pullRequestTemplate, c.config.Repo, nil)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to insert body into PR template")
		}
	}
	resp, err := c.api.CreatePullRequest(ctx, genclient.CreatePullRequestInput{
		RepositoryId: info.RepositoryID,
		BaseRefName:  baseRefName,
		HeadRefName:  headRefName,
		Title:        commit.Subject,
		Body:         &body,
		Draft:        &c.config.User.CreateDraftPRs,
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
		fmt.Printf("> github create %d : %s\n", pr.Number, pr.Title)
	}

	return pr
}

func formatStackMarkdown(commit git.Commit, stack []*github.PullRequest) string {
	var buf bytes.Buffer
	for i := len(stack) - 1; i >= 0; i-- {
		isCurrent := stack[i].Commit == commit
		var suffix string
		if isCurrent {
			suffix = " ⬅"
		} else {
			suffix = ""
		}
		buf.WriteString(fmt.Sprintf("- #%d%s\n", stack[i].Number, suffix))
	}

	return buf.String()
}

func formatBody(commit git.Commit, stack []*github.PullRequest) string {
	if len(stack) <= 1 {
		return strings.TrimSpace(commit.Body)
	}

	if commit.Body == "" {
		return fmt.Sprintf("**Stack**:\n%s",
			addManualMergeNotice(formatStackMarkdown(commit, stack)))
	}

	return fmt.Sprintf("%s\n\n---\n\n**Stack**:\n%s",
		commit.Body,
		addManualMergeNotice(formatStackMarkdown(commit, stack)))
}

// Reads the specified PR template file and returns it as a string
func readPRTemplate(gitcmd git.GitInterface, templatePath string) (string, error) {
	repoRootDir := gitcmd.RootDir()
	fullTemplatePath := filepath.Clean(path.Join(repoRootDir, templatePath))
	pullRequestTemplateBytes, err := os.ReadFile(fullTemplatePath)
	if err != nil {
		return "", fmt.Errorf("%w: unable to read template %v", err, fullTemplatePath)
	}
	return string(pullRequestTemplateBytes), nil
}

// insertBodyIntoPRTemplate inserts a text body into the given PR template and returns the result as a string.
// It uses the PRTemplateInsertStart and PRTemplateInsertEnd values defined in RepoConfig to determine where the body
// should be inserted in the PR template. If there are issues finding the correct place to insert the body
// an error will be returned.
//
// NOTE: on PR update, rather than using the PR template, it will use the existing PR body, which should have
// the PR template from the initial PR create.
func insertBodyIntoPRTemplate(body, prTemplate string, repo *config.RepoConfig, pr *github.PullRequest) (string, error) {
	templateOrExistingPRBody := prTemplate
	if pr != nil && pr.Body != "" {
		templateOrExistingPRBody = pr.Body
	}

	startPRTemplateSection, err := getSectionOfPRTemplate(templateOrExistingPRBody, repo.PRTemplateInsertStart, BeforeMatch)
	if err != nil {
		return "", fmt.Errorf("%w: PR template insert start = '%v'", err, repo.PRTemplateInsertStart)
	}

	endPRTemplateSection, err := getSectionOfPRTemplate(templateOrExistingPRBody, repo.PRTemplateInsertEnd, AfterMatch)
	if err != nil {
		return "", fmt.Errorf("%w: PR template insert end = '%v'", err, repo.PRTemplateInsertStart)
	}

	return fmt.Sprintf("%v%v\n%v\n\n%v%v", startPRTemplateSection, repo.PRTemplateInsertStart, body,
		repo.PRTemplateInsertEnd, endPRTemplateSection), nil
}

const (
	BeforeMatch = iota
	AfterMatch
)

// getSectionOfPRTemplate searches text for a matching searchString and will return the text before or after the
// match as a string. If there are no matches or more than one match is found, an error will be returned.
func getSectionOfPRTemplate(text, searchString string, returnMatch int) (string, error) {
	split := strings.Split(text, searchString)
	switch len(split) {
	case 2:
		if returnMatch == BeforeMatch {
			return split[0], nil
		} else if returnMatch == AfterMatch {
			return split[1], nil
		}
		return "", fmt.Errorf("invalid enum value")
	case 1:
		return "", fmt.Errorf("no matches found")
	default:
		return "", fmt.Errorf("multiple matches found")
	}
}

func addManualMergeNotice(body string) string {
	return body + "\n\n" +
		"⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). " +
		"Do not merge manually using the UI - doing so may have unexpected results.*"
}

func (c *client) UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface,
	info *github.GitHubInfo, pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github update %d : %s\n", pr.Number, pr.Title)
	}

	baseRefName := git.GetRemoteBranchName(c.config.Repo, gitcmd)
	if prevCommit != nil {
		baseRefName = branchNameFromCommit(info, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("PR", pr).Msg("UpdatePullRequest")

	body := formatBody(commit, info.PullRequests)
	if c.config.Repo.PRTemplatePath != "" {
		pullRequestTemplate, err := readPRTemplate(gitcmd, c.config.Repo.PRTemplatePath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read PR template")
		}
		body, err = insertBodyIntoPRTemplate(body, pullRequestTemplate, c.config.Repo, pr)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to insert body into PR template")
		}
	}
	title := &commit.Subject

	input := genclient.UpdatePullRequestInput{
		PullRequestId: pr.ID,
		BaseRefName:   &baseRefName,
		Title:         title,
		Body:          &body,
	}

	if c.config.User.PreserveTitleAndBody {
		input.Title = nil
		input.Body = nil
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
func (c *client) AddReviewers(ctx context.Context, pr *github.PullRequest, userIDs []string) {
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

func (c *client) CommentPullRequest(ctx context.Context, pr *github.PullRequest, comment string) {
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
	pr *github.PullRequest, mergeMethod genclient.PullRequestMergeMethod) {
	log.Debug().
		Interface("PR", pr).
		Str("mergeMethod", string(mergeMethod)).
		Msg("MergePullRequest")

	_, err := c.api.MergePullRequest(ctx, genclient.MergePullRequestInput{
		PullRequestId: pr.ID,
		MergeMethod:   &mergeMethod,
	})
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

func (c *client) ClosePullRequest(ctx context.Context, pr *github.PullRequest) {
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

func branchNameFromCommit(info *github.GitHubInfo, commit git.Commit) string {
	return "spr/" + info.LocalBranch + "/" + commit.CommitID
}

// sortPullRequests sorts the pull requests so that the one that is on top of
// the target branch will come first followed by the ones that are stacked on top.
// The stack order is maintained so that multiple pull requests can be merged in
// the correct order.
func sortPullRequests(prs []*github.PullRequest, config *config.Config, targetBranch string) []*github.PullRequest {
	swap := func(i int, j int) {
		buf := prs[i]
		prs[i] = prs[j]
		prs[j] = buf
	}

	j := 0
	for i := 0; i < len(prs); i++ {
		for j = i; j < len(prs); j++ {
			if prs[j].ToBranch == targetBranch {
				targetBranch = prs[j].FromBranch
				swap(i, j)
				break
			}
		}
	}

	// update stacked merge status flag
	for _, pr := range prs {
		if pr.Ready(config) {
			pr.MergeStatus.Stacked = true
		} else {
			break
		}
	}

	return prs
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
