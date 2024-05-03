package githubclient

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
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

func (c *client) GetInfo(ctx context.Context, gitcmd git.GitInterface) *github.GitHubInfo {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github fetch pull requests\n")
	}

	var pullRequestConnection fezzik_types.PullRequestConnection
	var loginName string
	var repoID string
	if c.config.Repo.MergeQueue {
		resp, err := c.api.PullRequestsWithMergeQueue(ctx,
			c.config.Repo.GitHubRepoOwner,
			c.config.Repo.GitHubRepoName)
		check(err)
		pullRequestConnection = resp.Viewer.PullRequests
		loginName = resp.Viewer.Login
		repoID = resp.Repository.Id
	} else {
		resp, err := c.api.PullRequests(ctx,
			c.config.Repo.GitHubRepoOwner,
			c.config.Repo.GitHubRepoName)
		check(err)
		pullRequestConnection = resp.Viewer.PullRequests
		loginName = resp.Viewer.Login
		repoID = resp.Repository.Id
	}

	targetBranch := c.config.Repo.GitHubBranch
	localCommitStack := git.GetLocalCommitStack(c.config, gitcmd)

	pullRequests := matchPullRequestStack(c.config.Repo, targetBranch, localCommitStack, pullRequestConnection)
	for _, pr := range pullRequests {
		if pr.Ready(c.config) {
			pr.MergeStatus.Stacked = true
		} else {
			break
		}
	}

	info := &github.GitHubInfo{
		UserName:     loginName,
		RepositoryID: repoID,
		LocalBranch:  git.GetLocalBranchName(gitcmd),
		PullRequests: pullRequests,
	}

	log.Debug().Interface("Info", info).Msg("GetInfo")
	return info
}

func matchPullRequestStack(
	repoConfig *config.RepoConfig,
	targetBranch string,
	localCommitStack []git.Commit,
	allPullRequests fezzik_types.PullRequestConnection) []*github.PullRequest {

	if len(localCommitStack) == 0 || allPullRequests.Nodes == nil {
		return []*github.PullRequest{}
	}

	// pullRequestMap is a map from commit-id to pull request
	pullRequestMap := make(map[string]*github.PullRequest)
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

		pullRequest := &github.PullRequest{
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

			checkStatus := github.CheckStatusPass
			if commit.StatusCheckRollup != nil {
				switch commit.StatusCheckRollup.State {
				case "SUCCESS":
					checkStatus = github.CheckStatusPass
				case "PENDING":
					checkStatus = github.CheckStatusPending
				default:
					checkStatus = github.CheckStatusFail
				}
			}

			pullRequest.MergeStatus = github.PullRequestMergeStatus{
				ChecksPass:     checkStatus,
				ReviewApproved: node.ReviewDecision != nil && *node.ReviewDecision == "APPROVED",
				NoConflicts:    node.Mergeable == "MERGEABLE",
			}

			pullRequestMap[pullRequest.Commit.CommitID] = pullRequest
		}
	}

	var pullRequests []*github.PullRequest

	// find top pr
	var currpr *github.PullRequest
	var found bool
	for i := len(localCommitStack) - 1; i >= 0; i-- {
		currpr, found = pullRequestMap[localCommitStack[i].CommitID]
		if found {
			break
		}
	}

	// The list of commits from the command line actually starts at the
	//  most recent commit. In order to reverse the list we use a
	//  custom prepend function instead of append
	prepend := func(l []*github.PullRequest, pr *github.PullRequest) []*github.PullRequest {
		l = append(l, &github.PullRequest{})
		copy(l[1:], l)
		l[0] = pr
		return l
	}

	// build pr stack
	for currpr != nil {
		pullRequests = prepend(pullRequests, currpr)
		if currpr.ToBranch == targetBranch {
			break
		}

		matches := git.BranchNameRegex.FindStringSubmatch(currpr.ToBranch)
		if matches == nil {
			panic(fmt.Errorf("invalid base branch for pull request:%s", currpr.ToBranch))
		}
		nextCommitID := matches[2]

		currpr = pullRequestMap[nextCommitID]
	}

	return pullRequests
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

	baseRefName := c.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(c.config, *prevCommit)
	}
	headRefName := git.BranchNameFromCommit(c.config, commit)

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", headRefName).Str("ToBranch", baseRefName).
		Msg("CreatePullRequest")

	body := formatBody(commit, info.PullRequests, c.config.Repo.ShowPrTitlesInStack)
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

func formatStackMarkdown(commit git.Commit, stack []*github.PullRequest, showPrTitlesInStack bool) string {
	var buf bytes.Buffer
	for i := len(stack) - 1; i >= 0; i-- {
		isCurrent := stack[i].Commit == commit
		var suffix string
		if isCurrent {
			suffix = " ⬅"
		} else {
			suffix = ""
		}
		var prTitle string
		if showPrTitlesInStack {
			prTitle = fmt.Sprintf("%s ", stack[i].Title)
		} else {
			prTitle = ""
		}

		buf.WriteString(fmt.Sprintf("- %s#%d%s\n", prTitle, stack[i].Number, suffix))
	}

	return buf.String()
}

func formatBody(commit git.Commit, stack []*github.PullRequest, showPrTitlesInStack bool) string {
	if len(stack) <= 1 {
		return strings.TrimSpace(commit.Body)
	}

	if commit.Body == "" {
		return fmt.Sprintf("**Stack**:\n%s",
			addManualMergeNotice(formatStackMarkdown(commit, stack, showPrTitlesInStack)))
	}

	return fmt.Sprintf("%s\n\n---\n\n**Stack**:\n%s",
		commit.Body,
		addManualMergeNotice(formatStackMarkdown(commit, stack, showPrTitlesInStack)))
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

func (c *client) UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface, pullRequests []*github.PullRequest, pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github update %d : %s\n", pr.Number, pr.Title)
	}

	baseRefName := c.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(c.config, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("PR", pr).Msg("UpdatePullRequest")

	body := formatBody(commit, pullRequests, c.config.Repo.ShowPrTitlesInStack)
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
		Title:         title,
		Body:          &body,
	}

	if !pr.InQueue {
		input.BaseRefName = &baseRefName
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

	var err error
	if c.config.Repo.MergeQueue {
		_, err = c.api.AutoMergePullRequest(ctx, genclient.EnablePullRequestAutoMergeInput{
			PullRequestId: pr.ID,
			MergeMethod:   &mergeMethod,
		})
	} else {
		_, err = c.api.MergePullRequest(ctx, genclient.MergePullRequestInput{
			PullRequestId: pr.ID,
			MergeMethod:   &mergeMethod,
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
