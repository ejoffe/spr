package githubclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/fezzik_types"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/ejoffe/spr/github/template/config_fetcher"
	"github.com/rs/zerolog/log"
	"github.com/zalando/go-keyring"
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
				secret, err := keyring.Get("gh:github.com", user.User)
				if err != nil {
					return user.OauthToken
				}
				return secret
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
	var endpoint string
	if strings.HasSuffix(config.Repo.GitHubHost, "github.com") {
		endpoint = "https://api.github.com/graphql"
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
		endpoint = fmt.Sprintf("%s://%s/api/graphql", scheme, host)
	}
	api = genclient.NewClient(endpoint, tc)
	return &client{
		config:          config,
		api:             api,
		graphqlEndpoint: endpoint,
		httpClient:      tc,
	}
}

type client struct {
	config          *config.Config
	api             genclient.Client
	graphqlEndpoint string
	httpClient      *http.Client
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

	pullRequests := matchPullRequestStack(c.config.Repo, c.config.BranchPrefix(), targetBranch, localCommitStack, pullRequestConnection)

	// When RequiredChecks is explicitly configured, fetch individual check contexts
	// and only evaluate the listed checks. This allows non-required check failures
	// to be ignored. When RequiredChecks is not set, the statusCheckRollup.state
	// from the fezzik query is used as-is (all checks matter).
	if c.config.Repo.RequireChecks && len(c.config.Repo.RequiredChecks) > 0 && len(pullRequests) > 0 {
		requiredStatus := c.fetchRequiredChecksStatus(ctx, pullRequests)
		if requiredStatus != nil {
			for _, pr := range pullRequests {
				if status, ok := requiredStatus[pr.Number]; ok {
					pr.MergeStatus.ChecksPass = status
				}
			}
		}
	}

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
	branchPrefix string,
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

		matches := git.BranchNameRegex(branchPrefix).FindStringSubmatch(node.HeadRefName)
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

	// store local commit hashes on PRs for display purposes
	//  (the remote CommitHash is preserved for update detection in syncCommitStackToGitHub)
	for _, c := range localCommitStack {
		if pr, ok := pullRequestMap[c.CommitID]; ok && c.CommitHash != "" {
			pr.LocalCommitHash = c.CommitHash
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

		matches := git.BranchNameRegex(branchPrefix).FindStringSubmatch(currpr.ToBranch)
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

	pr := &github.PullRequest{
		ID:         resp.CreatePullRequest.PullRequest.Id,
		Number:     resp.CreatePullRequest.PullRequest.Number,
		FromBranch: headRefName,
		ToBranch:   baseRefName,
		Commit:     commit,
		Title:      commit.Subject,
		Body:       resp.CreatePullRequest.PullRequest.Body,
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

func (c *client) UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface,
	info *github.GitHubInfo, pullRequests []*github.PullRequest, pr *github.PullRequest,
	commit git.Commit, prevCommit *git.Commit) {

	baseRefName := c.config.Repo.GitHubBranch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(c.config, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("PR", pr).Msg("UpdatePullRequest")

	templatizer := config_fetcher.PRTemplatizer(c.config, gitcmd)
	title := templatizer.Title(info, commit)
	body := templatizer.Body(info, commit, pr)

	// Skip the API call if nothing has actually changed. This avoids
	// triggering a spurious pull_request "edited" event on GitHub which
	// would cause a second (duplicate) Actions run after the force-push
	// already triggered a "synchronize" event.
	titleUnchanged := c.config.User.PreserveTitleAndBody || title == pr.Title
	bodyUnchanged := c.config.User.PreserveTitleAndBody || body == pr.Body
	baseUnchanged := pr.InQueue || baseRefName == pr.ToBranch
	if titleUnchanged && bodyUnchanged && baseUnchanged {
		log.Debug().Int("number", pr.Number).Msg("UpdatePullRequest: skipping, nothing changed")
		if c.config.User.LogGitHubCalls {
			fmt.Printf("> github update %d : %s (skipped, no changes)\n", pr.Number, pr.Title)
		}
		return
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github update %d : %s\n", pr.Number, pr.Title)
	}

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

// Response types for the raw GraphQL query that fetches individual check contexts.
// These are used instead of fezzik-generated types because fezzik does not support
// inline fragments on union types (StatusCheckRollupContext = CheckRun | StatusContext).

type checkContextNode struct {
	TypeName   string  `json:"__typename"`
	Name       string  `json:"name"`       // CheckRun
	Conclusion *string `json:"conclusion"` // CheckRun (nil when not completed)
	Status     string  `json:"status"`     // CheckRun: COMPLETED, IN_PROGRESS, QUEUED, etc.
	Context    string  `json:"context"`    // StatusContext
	State      string  `json:"state"`      // StatusContext: SUCCESS, FAILURE, PENDING, etc.
}

type checkContextsResult struct {
	Number  int `json:"number"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					Contexts struct {
						Nodes []checkContextNode `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

type graphqlRequest struct {
	Query string `json:"query"`
}

type graphqlResponse struct {
	Data   map[string]json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// fetchRequiredChecksStatus makes a single batched GraphQL query to fetch
// individual check contexts for all given pull requests. It evaluates only
// the checks listed in config.Repo.RequiredChecks and returns a map from
// PR number to the computed status.
func (c *client) fetchRequiredChecksStatus(ctx context.Context, pullRequests []*github.PullRequest) map[int]github.CheckStatus {
	if len(pullRequests) == 0 {
		return nil
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> github fetch required check status\n")
	}

	// Build a single GraphQL query with one aliased field per PR.
	var queryBuilder strings.Builder
	queryBuilder.WriteString("query {")
	for _, pr := range pullRequests {
		fmt.Fprintf(&queryBuilder, `
  pr_%d: node(id: %q) {
    ... on PullRequest {
      number
      commits(last: 1) {
        nodes {
          commit {
            statusCheckRollup {
              contexts(first: 100) {
                nodes {
                  __typename
                  ... on CheckRun {
                    name
                    conclusion
                    status
                  }
                  ... on StatusContext {
                    context
                    state
                  }
                }
              }
            }
          }
        }
      }
    }
  }`, pr.Number, pr.ID)
	}
	queryBuilder.WriteString("\n}")

	reqBody, err := json.Marshal(graphqlRequest{
		Query: queryBuilder.String(),
	})
	if err != nil {
		log.Warn().Err(err).Msg("failed to marshal required checks query")
		return nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphqlEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		log.Warn().Err(err).Msg("failed to create required checks request")
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Accept", "application/json; charset=utf-8")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch required checks status")
		return nil
	}
	defer httpResp.Body.Close()

	var gqlResp graphqlResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&gqlResp); err != nil {
		log.Warn().Err(err).Msg("failed to decode required checks response")
		return nil
	}
	if len(gqlResp.Errors) > 0 {
		log.Warn().Str("error", gqlResp.Errors[0].Message).Msg("graphql error fetching required checks")
		return nil
	}

	// Build the set of required check names from config.
	requiredSet := make(map[string]bool, len(c.config.Repo.RequiredChecks))
	for _, name := range c.config.Repo.RequiredChecks {
		requiredSet[name] = true
	}

	result := make(map[int]github.CheckStatus)
	for _, pr := range pullRequests {
		alias := fmt.Sprintf("pr_%d", pr.Number)
		raw, ok := gqlResp.Data[alias]
		if !ok {
			continue
		}
		var prResult checkContextsResult
		if err := json.Unmarshal(raw, &prResult); err != nil {
			log.Warn().Err(err).Int("pr", pr.Number).Msg("failed to unmarshal check contexts for PR")
			continue
		}
		if len(prResult.Commits.Nodes) == 0 {
			continue
		}
		commit := prResult.Commits.Nodes[0].Commit
		if commit.StatusCheckRollup == nil {
			// No checks configured — treat as pass
			result[pr.Number] = github.CheckStatusPass
			continue
		}
		result[pr.Number] = computeRequiredCheckStatus(commit.StatusCheckRollup.Contexts.Nodes, requiredSet)
	}

	return result
}

// contextName returns the display name for a check context node.
// For CheckRun nodes this is the Name field; for StatusContext nodes it's the Context field.
func contextName(ctx checkContextNode) string {
	if ctx.TypeName == "StatusContext" {
		return ctx.Context
	}
	return ctx.Name
}

// computeRequiredCheckStatus determines the aggregate check status considering
// only the checks whose name/context appears in requiredChecks.
// If a required check hasn't reported yet (not present in contexts), it is
// treated as pending.
func computeRequiredCheckStatus(contexts []checkContextNode, requiredChecks map[string]bool) github.CheckStatus {
	// Track which required checks we've seen
	seen := make(map[string]bool, len(requiredChecks))
	hasPending := false
	hasFail := false

	for _, ctx := range contexts {
		name := contextName(ctx)
		if !requiredChecks[name] {
			continue
		}
		seen[name] = true

		switch ctx.TypeName {
		case "CheckRun":
			switch ctx.Status {
			case "COMPLETED":
				if ctx.Conclusion == nil {
					hasFail = true
				} else {
					switch *ctx.Conclusion {
					case "SUCCESS", "NEUTRAL", "SKIPPED":
						// pass
					default:
						hasFail = true
					}
				}
			default:
				// IN_PROGRESS, QUEUED, REQUESTED, WAITING, PENDING
				hasPending = true
			}
		case "StatusContext":
			switch ctx.State {
			case "SUCCESS":
				// pass
			case "PENDING", "EXPECTED":
				hasPending = true
			default:
				hasFail = true
			}
		}
	}

	// Any required check that hasn't reported yet is pending
	for name := range requiredChecks {
		if !seen[name] {
			hasPending = true
		}
	}

	if hasFail {
		return github.CheckStatusFail
	}
	if hasPending {
		return github.CheckStatusPending
	}
	return github.CheckStatusPass
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
