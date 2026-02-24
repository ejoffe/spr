package gitlabclient

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/forge"
	"github.com/ejoffe/spr/forge/template/config_fetcher"
	"github.com/ejoffe/spr/git"
	"github.com/rs/zerolog/log"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gopkg.in/yaml.v3"
)

// glab cli config (https://gitlab.com/gitlab-org/cli)
type glabCLIConfig struct {
	Host  string `yaml:"host"`
	Hosts map[string]struct {
		Token       string `yaml:"token"`
		APIHost     string `yaml:"api_host"`
		GitProtocol string `yaml:"git_protocol"`
		APIProtocol string `yaml:"api_protocol"`
	} `yaml:"hosts"`
}

func readGlabCLIConfig() (*glabCLIConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	f, err := os.Open(path.Join(homeDir, ".config", "glab-cli", "config.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to open glab cli config file: %w", err)
	}

	var cfg glabCLIConfig
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse glab cli config file: %w", err)
	}

	return &cfg, nil
}

func findToken(gitlabHost string) string {
	// Try environment variable first
	token := os.Getenv("GITLAB_TOKEN")
	if token != "" {
		return token
	}

	token = os.Getenv("GITLAB_PRIVATE_TOKEN")
	if token != "" {
		return token
	}

	// Try ~/.config/glab-cli/config.yml
	cfg, err := readGlabCLIConfig()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read glab cli config file")
	} else {
		for host, hostCfg := range cfg.Hosts {
			if host == gitlabHost {
				return hostCfg.Token
			}
		}
	}

	return ""
}

const tokenHelpText = `
No GitLab API token found! Create a personal access token
at https://%s/-/user_settings/personal_access_tokens
with the "api" scope, then either set the GITLAB_TOKEN environment variable:

	$ export GITLAB_TOKEN=<your token>

or use the official "glab" CLI (https://gitlab.com/gitlab-org/cli) to log in:

	$ glab auth login
`

func NewGitLabClient(ctx context.Context, cfg *config.Config) *client {
	token := findToken(cfg.Repo.ForgeHost)
	if token == "" {
		fmt.Printf(tokenHelpText, cfg.Repo.ForgeHost)
		os.Exit(3)
	}

	var baseURL string
	if strings.Contains(cfg.Repo.ForgeHost, "://") {
		baseURL = cfg.Repo.ForgeHost
	} else {
		baseURL = "https://" + cfg.Repo.ForgeHost
	}

	api, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL+"/api/v4"))
	if err != nil {
		fmt.Printf("error: failed to create GitLab client: %s\n", err)
		os.Exit(3)
	}

	return &client{
		config:    cfg,
		api:       api,
		projectID: cfg.Repo.RepoOwner + "/" + cfg.Repo.RepoName,
	}
}

type client struct {
	config    *config.Config
	api       *gitlab.Client
	projectID string
}

func (c *client) GetInfo(ctx context.Context, gitcmd git.GitInterface) *forge.ForgeInfo {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab fetch merge requests\n")
	}

	user, _, err := c.api.Users.CurrentUser()
	check(err)

	project, _, err := c.api.Projects.GetProject(c.projectID, nil)
	check(err)

	targetBranch := c.config.Repo.Branch
	localCommitStack := git.GetLocalCommitStack(c.config, gitcmd)

	state := "opened"
	scope := "all"
	authorID := int64(user.ID)
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State:    &state,
		Scope:    &scope,
		AuthorID: &authorID,
	}
	mrs, _, err := c.api.MergeRequests.ListProjectMergeRequests(c.projectID, opts)
	check(err)

	pullRequests := matchMergeRequestStack(c.config.Repo, targetBranch, localCommitStack, mrs)
	for _, pr := range pullRequests {
		approvals, _, err := c.api.MergeRequestApprovals.GetConfiguration(c.projectID, int64(pr.Number))
		if err != nil {
			log.Warn().Err(err).Int("mr", pr.Number).Msg("failed to get merge request approvals")
		} else {
			pr.MergeStatus.ReviewApproved = len(approvals.ApprovedBy) > 0
		}
	}
	for _, pr := range pullRequests {
		commits, _, err := c.api.MergeRequests.GetMergeRequestCommits(c.projectID, int64(pr.Number), nil)
		if err != nil {
			log.Warn().Err(err).Int("mr", pr.Number).Msg("failed to get merge request commits")
		} else {
			var prCommits []git.Commit
			for _, commit := range commits {
				for _, line := range strings.Split(commit.Message, "\n") {
					if strings.HasPrefix(line, "commit-id:") {
						prCommits = append(prCommits, git.Commit{
							CommitID:   strings.Split(line, ":")[1],
							CommitHash: commit.ID,
							Subject:    commit.Title,
							Body:       commit.Message,
						})
					}
				}
			}
			pr.Commits = prCommits
		}
	}
	for _, pr := range pullRequests {
		if pr.Ready(c.config) {
			pr.MergeStatus.Stacked = true
		} else {
			break
		}
	}

	info := &forge.ForgeInfo{
		UserName:       user.Username,
		RepositoryID:   fmt.Sprintf("%d", project.ID),
		LocalBranch:    git.GetLocalBranchName(gitcmd),
		PullRequests:   pullRequests,
		PRNumberPrefix: "!",
	}

	log.Debug().Interface("Info", info).Msg("GetInfo")
	return info
}

func matchMergeRequestStack(
	repoConfig *config.RepoConfig,
	targetBranch string,
	localCommitStack []git.Commit,
	allMergeRequests []*gitlab.BasicMergeRequest) []*forge.PullRequest {

	if len(localCommitStack) == 0 || len(allMergeRequests) == 0 {
		return []*forge.PullRequest{}
	}

	pullRequestMap := make(map[string]*forge.PullRequest)
	for _, mr := range allMergeRequests {
		matches := git.BranchNameRegex.FindStringSubmatch(mr.SourceBranch)
		if matches != nil {
			commitID := matches[2]

			checkStatus := forge.CheckStatusUnknown
			switch mr.DetailedMergeStatus {
			case "mergeable", "ci_must_pass":
				checkStatus = forge.CheckStatusPass
			case "ci_still_running", "checking", "preparing":
				checkStatus = forge.CheckStatusPending
			case "broken_status", "not_approved", "blocked_status":
				checkStatus = forge.CheckStatusFail
			}

			pr := &forge.PullRequest{
				ID:         fmt.Sprintf("%d", mr.IID),
				Number:     int(mr.IID),
				Title:      mr.Title,
				Body:       mr.Description,
				FromBranch: mr.SourceBranch,
				ToBranch:   mr.TargetBranch,
				Commit: git.Commit{
					CommitID: commitID,
					Subject:  mr.Title,
				},
				MergeStatus: forge.PullRequestMergeStatus{
					ChecksPass:  checkStatus,
					NoConflicts: !mr.HasConflicts,
				},
			}

			pullRequestMap[commitID] = pr
		}
	}

	return forge.BuildPullRequestStack(targetBranch, localCommitStack, pullRequestMap)
}

func (c *client) GetAssignableUsers(ctx context.Context) []forge.RepoAssignee {
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab get project members\n")
	}

	users := []forge.RepoAssignee{}
	opts := &gitlab.ListProjectMembersOptions{}
	members, _, err := c.api.ProjectMembers.ListAllProjectMembers(c.projectID, opts)
	if err != nil {
		log.Fatal().Err(err).Msg("get project members failed")
		return nil
	}

	for _, member := range members {
		users = append(users, forge.RepoAssignee{
			ID:    fmt.Sprintf("%d", member.ID),
			Login: member.Username,
			Name:  member.Name,
		})
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
	title := templatizer.Title(info, commit)
	if c.config.User.CreateDraftPRs {
		title = "Draft: " + title
	}
	body := templatizer.Body(info, commit, nil)
	removeSourceBranch := true
	opts := &gitlab.CreateMergeRequestOptions{
		SourceBranch:       &headRefName,
		TargetBranch:       &baseRefName,
		Title:              &title,
		Description:        &body,
		RemoveSourceBranch: &removeSourceBranch,
	}
	mr, _, err := c.api.MergeRequests.CreateMergeRequest(c.projectID, opts)
	check(err)

	pr := &forge.PullRequest{
		ID:         fmt.Sprintf("%d", mr.IID),
		Number:     int(mr.IID),
		FromBranch: headRefName,
		ToBranch:   baseRefName,
		Commit:     commit,
		Title:      commit.Subject,
		Body:       mr.Description,
		MergeStatus: forge.PullRequestMergeStatus{
			ChecksPass:     forge.CheckStatusUnknown,
			ReviewApproved: false,
			NoConflicts:    false,
			Stacked:        false,
		},
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab create !%d : %s\n", pr.Number, pr.Title)
	}

	return pr
}

func (c *client) UpdatePullRequest(ctx context.Context, gitcmd git.GitInterface,
	info *forge.ForgeInfo, pullRequests []*forge.PullRequest, pr *forge.PullRequest,
	commit git.Commit, prevCommit *git.Commit) {

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab update !%d : %s\n", pr.Number, pr.Title)
	}

	baseRefName := c.config.Repo.Branch
	if prevCommit != nil {
		baseRefName = git.BranchNameFromCommit(c.config, *prevCommit)
	}

	log.Debug().Interface("Commit", commit).
		Str("FromBranch", pr.FromBranch).Str("ToBranch", baseRefName).
		Interface("MR", pr).Msg("UpdatePullRequest")

	templatizer := config_fetcher.PRTemplatizer(c.config, gitcmd)
	title := templatizer.Title(info, commit)
	body := templatizer.Body(info, commit, pr)
	opts := &gitlab.UpdateMergeRequestOptions{
		Title:       &title,
		Description: &body,
	}
	if c.config.User.PreserveTitleAndBody {
		opts.Title = nil
		opts.Description = nil
	}

	if !pr.InQueue {
		opts.TargetBranch = &baseRefName
	}

	_, _, err := c.api.MergeRequests.UpdateMergeRequest(c.projectID, int64(pr.Number), opts)
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("merge request update failed")
	}
}

func (c *client) AddReviewers(ctx context.Context, pr *forge.PullRequest, userIDs []string) {
	log.Debug().Strs("userIDs", userIDs).Msg("AddReviewers")
	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab add reviewers !%d : %s - %+v\n", pr.Number, pr.Title, userIDs)
	}

	reviewerIDs := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		intID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			log.Warn().Str("id", id).Err(err).Msg("invalid reviewer ID")
			continue
		}
		reviewerIDs = append(reviewerIDs, intID)
	}

	opts := &gitlab.UpdateMergeRequestOptions{
		ReviewerIDs: &reviewerIDs,
	}
	_, _, err := c.api.MergeRequests.UpdateMergeRequest(c.projectID, int64(pr.Number), opts)
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
	opts := &gitlab.CreateMergeRequestNoteOptions{
		Body: &comment,
	}
	_, _, err := c.api.Notes.CreateMergeRequestNote(c.projectID, int64(pr.Number), opts)
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("merge request comment failed")
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab add comment !%d : %s\n", pr.Number, pr.Title)
	}
}

func (c *client) MergePullRequest(ctx context.Context,
	pr *forge.PullRequest, mergeMethod config.MergeMethod) {
	log.Debug().
		Interface("MR", pr).
		Str("mergeMethod", string(mergeMethod)).
		Msg("MergePullRequest")

	squash := mergeMethod == config.MergeMethodSquash
	opts := &gitlab.AcceptMergeRequestOptions{
		Squash: &squash,
	}

	if mergeMethod == config.MergeMethodRebase {
		_, err := c.api.MergeRequests.RebaseMergeRequest(c.projectID, int64(pr.Number), nil)
		if err != nil {
			log.Warn().Err(err).Msg("rebase before merge failed, proceeding with merge")
		}
	}

	// GitLab recalculates MR mergeability asynchronously after target branch
	// changes and rebases. Retry on 405 (not yet mergeable) with backoff.
	const maxAttempts = 15
	const retryDelay = 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, resp, err := c.api.MergeRequests.AcceptMergeRequest(c.projectID, int64(pr.Number), opts)
		if err == nil {
			if c.config.User.LogGitHubCalls {
				fmt.Printf("> gitlab merge !%d : %s\n", pr.Number, pr.Title)
			}
			return
		}
		lastErr = err

		if resp == nil || resp.StatusCode != http.StatusMethodNotAllowed {
			break
		}

		if attempt < maxAttempts {
			log.Debug().
				Int("attempt", attempt).
				Int("maxAttempts", maxAttempts).
				Msg("merge request not yet mergeable, waiting")
			time.Sleep(retryDelay)
		}
	}

	log.Fatal().
		Str("id", pr.ID).
		Int("number", pr.Number).
		Str("title", pr.Title).
		Err(lastErr).
		Msg("merge request merge failed")
}

func (c *client) PullRequestURL(number int) string {
	return fmt.Sprintf("https://%s/%s/%s/-/merge_requests/%d",
		c.config.Repo.ForgeHost, c.config.Repo.RepoOwner, c.config.Repo.RepoName, number)
}

func (c *client) ClosePullRequest(ctx context.Context, pr *forge.PullRequest) {
	log.Debug().Interface("MR", pr).Msg("ClosePullRequest")
	stateEvent := "close"
	opts := &gitlab.UpdateMergeRequestOptions{
		StateEvent: &stateEvent,
	}
	_, _, err := c.api.MergeRequests.UpdateMergeRequest(c.projectID, int64(pr.Number), opts)
	if err != nil {
		log.Fatal().
			Str("id", pr.ID).
			Int("number", pr.Number).
			Str("title", pr.Title).
			Err(err).
			Msg("merge request close failed")
	}

	if c.config.User.LogGitHubCalls {
		fmt.Printf("> gitlab close !%d : %s\n", pr.Number, pr.Title)
	}
}

func check(err error) {
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "401") {
			errmsg := "error : 401 Unauthorized\n"
			errmsg += " make sure GITLAB_TOKEN env variable is set with a valid token,\n"
			errmsg += " or log in with: glab auth login\n"
			errmsg += " to create a token manually goto your GitLab instance settings/access_tokens\n"
			fmt.Fprint(os.Stderr, errmsg)
			os.Exit(-1)
		} else {
			panic(err)
		}
	}
}
