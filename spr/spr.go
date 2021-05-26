package spr

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/ejoffe/profiletimer"
	"github.com/rs/zerolog/log"
	"github.com/shurcooL/githubv4"
)

// NewStackedPR constructs and returns a new instance stackediff.
func NewStackedPR(config *Config, github *githubv4.Client, writer io.Writer, debug bool) *stackediff {
	if debug {
		return &stackediff{
			config:       config,
			github:       github,
			writer:       writer,
			debug:        true,
			profiletimer: profiletimer.StartProfileTimer(),
		}
	}

	return &stackediff{
		config:       config,
		github:       github,
		writer:       writer,
		debug:        false,
		profiletimer: profiletimer.StartNoopTimer(),
	}
}

// AmendCommit enables one to easily ammend a commit in the middle of a stack
//  of commits. A list of commits is printed and one can be chosen to be ammended.
func (sd *stackediff) AmendCommit(ctx context.Context) {
	localCommits := sd.getLocalCommitStack(false)
	if len(localCommits) == 0 {
		fmt.Fprintf(sd.writer, "No commits to amend\n")
		return
	}

	for i := len(localCommits) - 1; i >= 0; i-- {
		commit := localCommits[i]
		fmt.Fprintf(sd.writer, " %d : %s : %s\n", i+1, commit.CommitID[0:8], commit.Subject)
	}

	if len(localCommits) == 1 {
		fmt.Fprintf(sd.writer, "Commit to amend [%d]: ", 1)
	} else {
		fmt.Fprintf(sd.writer, "Commit to amend [%d-%d]: ", 1, len(localCommits))
	}

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	commitIndex, err := strconv.Atoi(line)
	if err != nil {
		fmt.Fprint(sd.writer, "Invalid input\n")
		return
	}
	commitIndex = commitIndex - 1
	check(err)
	mustgit("commit --fixup "+localCommits[commitIndex].CommitHash, nil)
	mustgit("rebase origin/master -i --autosquash --autostash", nil)
}

// UpdatePullRequests implaments a stacked diff workflow on top of github.
//  Each time it's called it compares the local branch unmerged commits
//   with currently open pull requests in github.
//  It will create a new pull request for all new commits, and update the
//   pull request if a commit has been amended.
func (sd *stackediff) UpdatePullRequests(ctx context.Context) {
	sd.profiletimer.Step("UpdatePullRequests::Start")
	githubInfo := sd.fetchAndGetGitHubInfo(ctx, sd.github)
	sd.profiletimer.Step("UpdatePullRequests::FetchAndGetGitHubInfo")
	localCommits := sd.syncCommitStackToGitHub(ctx, githubInfo)
	sd.profiletimer.Step("UpdatePullRequests::SyncCommits")

	// iterate through local_commits and update pull_requests
	for commitIndex, c := range localCommits {
		prFound := false
		for _, pr := range githubInfo.PullRequests {
			if c.CommitID == pr.Commit.CommitID {
				prFound = true
				if c.CommitHash != pr.Commit.CommitHash {
					// if commit id is same but commit hash changed it means the commit
					//  has been amended and we need to update the pull request
					var prevCommit *commit
					if commitIndex > 0 {
						prevCommit = &localCommits[commitIndex-1]
					}
					updateGithubPullRequest(
						ctx, sd.github, githubInfo,
						pr, c, prevCommit)
				}
				break
			}
		}
		if !prFound {
			// if pull request is not found for this commit_id it means the commit
			//  is new and we need to create a new pull request
			var prevCommit *commit
			if commitIndex > 0 {
				prevCommit = &localCommits[commitIndex-1]
			}
			pr := createGithubPullRequest(
				ctx, sd.github, githubInfo,
				c, prevCommit)
			githubInfo.PullRequests = append(githubInfo.PullRequests, pr)
		}
	}

	githubInfo.PullRequests = sd.sortPullRequests(githubInfo.PullRequests)
	for i := len(githubInfo.PullRequests) - 1; i >= 0; i-- {
		pr := githubInfo.PullRequests[i]
		fmt.Fprintf(sd.writer, "%s\n", pr.String(sd.config))
	}
	sd.profiletimer.Step("UpdatePullRequests::End")
}

// MergePullRequests will go through all the current pull requests
//  and merge all requests that are mergeable.
// For a pull request to be mergeable it has to:
//  - have at least one approver
//  - pass all checks
//  - have no merge conflicts
//  - not be on top of another unmergable request
// In order to merge a stack of pull requests without generating conflicts
//  and other pr issues. We find the top mergeable pull request in the stack,
//  than we change this pull requests base to be master and than merge the
//  pull request. This one merge in effect merges all the commits in the stack.
//  We than close all the pull requests which are below the merged request, as
//  their commits have already been merged.
func (sd *stackediff) MergePullRequests(ctx context.Context) {
	sd.profiletimer.Step("MergePullRequests::Start")
	githubInfo := sd.getGitHubInfo(ctx, sd.github)
	sd.profiletimer.Step("MergePullRequests::getGitHubInfo")

	// Figure out top most pr in the stack that is mergeable
	var prIndex int
	for prIndex = 0; prIndex < len(githubInfo.PullRequests); prIndex++ {
		pr := githubInfo.PullRequests[prIndex]
		if !pr.mergeable(sd.config) {
			break
		}
	}
	prIndex--
	if prIndex == -1 {
		return
	}
	prToMerge := githubInfo.PullRequests[prIndex]

	// Update the base of the merging pr to master
	var updatepr struct {
		UpdatePullRequest struct {
			PullRequest struct {
				Number int
			}
		} `graphql:"updatePullRequest(input: $input)"`
	}
	baseRefMaster := githubv4.String("master")
	updatePRInput := githubv4.UpdatePullRequestInput{
		PullRequestID: prToMerge.ID,
		BaseRefName:   &baseRefMaster,
	}
	err := sd.github.Mutate(ctx, &updatepr, updatePRInput, nil)
	if err != nil {
		log.Fatal().
			Str("id", prToMerge.ID).
			Int("number", prToMerge.Number).
			Str("title", prToMerge.Title).
			Err(err).
			Msg("pull request update failed")
	}
	sd.profiletimer.Step("MergePullRequests::update pr base")

	// Merge pull request
	var mergepr struct {
		MergePullRequest struct {
			PullRequest struct {
				Number int
			}
		} `graphql:"mergePullRequest(input: $input)"`
	}
	mergeMethod := githubv4.PullRequestMergeMethodRebase
	mergePRInput := githubv4.MergePullRequestInput{
		PullRequestID: prToMerge.ID,
		MergeMethod:   &mergeMethod,
	}
	err = sd.github.Mutate(ctx, &mergepr, mergePRInput, nil)
	if err != nil {
		log.Fatal().
			Str("id", prToMerge.ID).
			Int("number", prToMerge.Number).
			Str("title", prToMerge.Title).
			Err(err).
			Msg("pull request merge failed")
	}
	sd.profiletimer.Step("MergePullRequests::merge pr")

	// Close all the pull requests in the stack below the merged pr
	for i := 0; i < prIndex; i++ {
		pr := githubInfo.PullRequests[i]
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
		err = sd.github.Mutate(ctx, &closepr, closePRInput, nil)
		log.Debug().Int("number", closepr.ClosePullRequest.PullRequest.Number).Msg("closed pr")
		if err != nil {
			log.Fatal().
				Str("id", pr.ID).
				Int("number", pr.Number).
				Str("title", pr.Title).
				Err(err).
				Msg("pull request close failed")
		}
	}
	sd.profiletimer.Step("MergePullRequests::close prs")

	for i := 0; i <= prIndex; i++ {
		pr := githubInfo.PullRequests[i]
		fmt.Fprintf(sd.writer, "MERGED %d: %v\n", pr.Number, pr.Title)
	}

	sd.profiletimer.Step("MergePullRequests::End")
}

// StatusPullRequests fetches all the users pull requests from github and
//  prints out the status of each. It does not make any updates locally or
//  remotely on github.
func (sd *stackediff) StatusPullRequests(ctx context.Context) {
	sd.profiletimer.Step("StatusPullRequests::Start")
	githubInfo := sd.getGitHubInfo(ctx, sd.github)

	for i := len(githubInfo.PullRequests) - 1; i >= 0; i-- {
		pr := githubInfo.PullRequests[i]
		fmt.Fprintf(sd.writer, "%s\n", pr.String(sd.config))
	}
	sd.profiletimer.Step("StatusPullRequests::End")
}

// DebugPrintSummary prints debug info if debug mode is enabled.
func (sd *stackediff) DebugPrintSummary() {
	if sd.debug {
		err := sd.profiletimer.ShowResults()
		check(err)
	}
}

type commit struct {
	CommitID   string
	CommitHash string
	Subject    string
	Body       string
}

type pullRequest struct {
	ID         string
	Number     int
	FromBranch string
	ToBranch   string
	Commit     commit
	Title      string

	MergeStatus pullRequestMergeStatus
}

type checkStatus int

const (
	checkStatusUnknown checkStatus = iota
	checkStatusPending
	checkStatusPass
	checkStatusFail
)

type pullRequestMergeStatus struct {
	ChecksPass     checkStatus
	ReviewApproved bool
	NoConflicts    bool
	Stacked        bool
}

type gitHubInfo struct {
	UserName     string
	RepositoryID string
	LocalBranch  string
	PullRequests []*pullRequest
}

type stackediff struct {
	config       *Config
	github       *githubv4.Client
	writer       io.Writer
	debug        bool
	profiletimer profiletimer.Timer
}

// sortPullRequests sorts the pull requests so that the one that is on top of
//  master will come first followed by the ones that are stacked on top.
// The stack order is maintained so that multiple pull requests can be merged in
//  the correct order.
func (sd *stackediff) sortPullRequests(prs []*pullRequest) []*pullRequest {

	swap := func(i int, j int) {
		buf := prs[i]
		prs[i] = prs[j]
		prs[j] = buf
	}

	targetBranch := "master"
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
		if pr.ready(sd.config) {
			pr.MergeStatus.Stacked = true
		} else {
			break
		}
	}

	return prs
}

// getLocalCommitStack returns a list of unmerged commits
func (sd *stackediff) getLocalCommitStack(skipWIP bool) []commit {

	var commits []commit
	var commitLog string
	mustgit("log origin/master..HEAD", &commitLog)
	lines := strings.Split(commitLog, "\n")

	commitHashRegex := regexp.MustCompile(`^commit ([a-f0-9]{40})`)
	commitIDRegex := regexp.MustCompile(`commit-id\:([a-f0-9]{8})`)

	// The list of commits from the command line actually starts at the
	//  most recent tio commit. In order to reverse the list we use a
	//  custom prepend function instead of append
	prepend := func(l []commit, c commit) []commit {
		l = append(l, commit{})
		copy(l[1:], l)
		l[0] = c
		return l
	}

	subjectIndex := 0
	commitScanOn := false
	// commit_scan_on is set to true when the commit_hash is matched
	//  and turns false when the commit-id is matched
	//  the commit subject and body is always between the hash and id
	var scannedCommit commit

	for index, line := range lines {
		matches := commitHashRegex.FindStringSubmatch(line)
		if matches != nil {
			if commitScanOn {
				sd.printCommitInstallHelper()
			}
			commitScanOn = true
			scannedCommit = commit{
				CommitHash: matches[1],
			}
			subjectIndex = index + 4
		}

		matches = commitIDRegex.FindStringSubmatch(line)
		if matches != nil {
			commitScanOn = false
			scannedCommit.CommitID = matches[1]

			if skipWIP && strings.HasPrefix(scannedCommit.Subject, "WIP") {
				// if commit subject starts with "WIP", ignore it
			} else {
				commits = prepend(commits, scannedCommit)
			}
		}

		if commitScanOn {
			if index == subjectIndex {
				scannedCommit.Subject = strings.TrimSpace(line)
			} else if index == (subjectIndex+1) && line != "\n" {
				scannedCommit.Body += strings.TrimSpace(line) + "\n"
			} else if index > (subjectIndex + 1) {
				scannedCommit.Body += strings.TrimSpace(line) + "\n"
			}
		}
	}

	// if commitScanOn is true here it means there was a commit without
	//  a commit-id
	if commitScanOn {
		sd.printCommitInstallHelper()
		return nil
	}

	return commits
}

func (sd *stackediff) printCommitInstallHelper() {
	fmt.Fprintf(sd.writer, "A commit is missing a commit-id.\n")
	fmt.Fprintf(sd.writer, "This most likely means the commit-msg hook isn't installed.\n")
	fmt.Fprintf(sd.writer, "To install the hook run the following cmd in the repo root dir:\n")
	fmt.Fprintf(sd.writer, " > ln -s spr-commithook .git/hooks/commit-msg\n")
	fmt.Fprintf(sd.writer, "After installing the hook, you'll need to ammend your commits\n")
}

func (sd *stackediff) fetchAndGetGitHubInfo(ctx context.Context, client *githubv4.Client) *gitHubInfo {
	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	fetch := func() {
		mustgit("fetch", nil)
		mustgit("rebase origin/master --autostash", nil)
		waitgroup.Done()
	}

	go fetch()
	info := sd.getGitHubInfo(ctx, client)
	waitgroup.Wait()

	return info
}

func (sd *stackediff) getGitHubInfo(ctx context.Context, client *githubv4.Client) *gitHubInfo {
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
		"repo_owner": githubv4.String(sd.config.GitHubRepoOwner),
		"repo_name":  githubv4.String(sd.config.GitHubRepoName),
	}
	err := client.Query(ctx, &query, variables)
	check(err)

	var branchname string
	mustgit("branch --show-current", &branchname)

	var requests []*pullRequest
	for _, node := range query.Viewer.PullRequests.Nodes {
		if query.Repository.ID != node.Repository.ID {
			continue
		}
		pullRequest := &pullRequest{
			ID:         node.ID,
			Number:     node.Number,
			Title:      node.Title,
			FromBranch: node.HeadRefName,
			ToBranch:   node.BaseRefName,
		}

		commitIDRegex := regexp.MustCompile(`pr/[a-zA-Z0-9_.-]+/([a-zA-Z0-9_.-]+)/([a-f0-9]{8})$`)
		matches := commitIDRegex.FindStringSubmatch(node.HeadRefName)
		if matches != nil && matches[1] == branchname {
			pullRequest.Commit = commit{
				CommitID:   matches[2],
				CommitHash: node.Commits.Nodes[0].Commit.Oid,
			}

			checkStatus := checkStatusUnknown
			switch node.Commits.Nodes[0].Commit.StatusCheckRollup.State {

			case "SUCCESS":
				checkStatus = checkStatusPass
			case "PENDING":
				checkStatus = checkStatusPending
			default:
				checkStatus = checkStatusFail
			}

			pullRequest.MergeStatus = pullRequestMergeStatus{
				ChecksPass:     checkStatus,
				ReviewApproved: node.ReviewDecision == "APPROVED",
				NoConflicts:    node.Mergeable == "MERGEABLE",
			}

			requests = append(requests, pullRequest)
		}
	}

	requests = sd.sortPullRequests(requests)

	return &gitHubInfo{
		UserName:     query.Viewer.Login,
		RepositoryID: query.Repository.ID,
		LocalBranch:  branchname,
		PullRequests: requests,
	}
}

// syncCommitStackToGitHub gets all the local commits in the given branch
//  which are new (on top of origin/master) and creates a corresponding
//  branch on github for each commit.
func (sd *stackediff) syncCommitStackToGitHub(ctx context.Context, info *gitHubInfo) []commit {
	localCommits := sd.getLocalCommitStack(true)

	var output string
	mustgit("status --porcelain --untracked-files=no", &output)
	if output != "" {
		mustgit("stash", nil)
		defer mustgit("stash pop", nil)
	}
	defer mustgit("switch "+info.LocalBranch, nil)
	sd.profiletimer.Step("SyncCommitStack::GetLocalCommitStack")

	commitUpdated := func(c commit, info *gitHubInfo) bool {
		for _, pr := range info.PullRequests {
			if pr.Commit.CommitID == c.CommitID {
				if pr.Commit.CommitHash == c.CommitHash {
					return false
				} else {
					return true
				}
			}
		}
		return true
	}

	for _, commit := range localCommits {
		if commitUpdated(commit, info) {
			headRefName := branchNameFromCommit(info, commit)
			mustgit("checkout "+commit.CommitHash, nil)
			mustgit("switch -C "+headRefName, nil)
			mustgit("push --force --set-upstream origin "+headRefName, nil)
			mustgit("switch "+info.LocalBranch, nil)
			mustgit("branch -D "+headRefName, nil)
			sd.profiletimer.Step("SyncCommitStack::" + commit.CommitID)
		}
	}

	return localCommits
}

func createGithubPullRequest(ctx context.Context, client *githubv4.Client,
	info *gitHubInfo, commit commit, prevCommit *commit) *pullRequest {

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
	err := client.Mutate(ctx, &mutation, input, nil)
	check(err)

	return &pullRequest{
		ID:         mutation.CreatePullRequest.PullRequest.ID,
		Number:     mutation.CreatePullRequest.PullRequest.Number,
		FromBranch: baseRefName,
		ToBranch:   headRefName,
		Commit:     commit,
		Title:      commit.Subject,
		MergeStatus: pullRequestMergeStatus{
			ChecksPass:     checkStatusUnknown,
			ReviewApproved: false,
			NoConflicts:    false,
			Stacked:        false,
		},
	}
}

func updateGithubPullRequest(ctx context.Context, client *githubv4.Client,
	info *gitHubInfo, pullRequest *pullRequest,
	commit commit, prevCommit *commit) {

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
		PullRequestID: pullRequest.ID,
		BaseRefName:   &baseRefNameStr,
		Title:         &subject,
		Body:          &body,
	}
	err := client.Mutate(ctx, &mutation, input, nil)
	check(err)
}

func branchNameFromCommit(info *gitHubInfo, commit commit) string {
	return "pr/" + info.UserName + "/" + info.LocalBranch + "/" + commit.CommitID
}

func mustgit(argStr string, output *string) {
	err := git(argStr, output)
	check(err)
}

func git(argStr string, output *string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command
	args := strings.Split(argStr, " ")
	cmd := exec.Command("git", args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "EDITOR=/usr/bin/true")

	if output != nil {
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		*output = strings.TrimSpace(string(out))
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "git error: %s", string(out))
			return err
		}
	}
	return nil
}

func (pr *pullRequest) mergeable(config *Config) bool {
	if !pr.MergeStatus.NoConflicts {
		return false
	}
	if !pr.MergeStatus.Stacked {
		return false
	}
	if config.RequireChecks && pr.MergeStatus.ChecksPass != checkStatusPass {
		return false
	}
	if config.RequireApproval && !pr.MergeStatus.ReviewApproved {
		return false
	}
	return true
}

func (pr *pullRequest) ready(config *Config) bool {
	if !pr.MergeStatus.NoConflicts {
		return false
	}
	if config.RequireChecks && pr.MergeStatus.ChecksPass != checkStatusPass {
		return false
	}
	if config.RequireApproval && !pr.MergeStatus.ReviewApproved {
		return false
	}
	return true
}

const checkmark = "\xE2\x9C\x94"
const crossmark = "\xE2\x9C\x97"
const middledot = "\xC2\xB7"

func (pr *pullRequest) String(config *Config) string {
	statusString := "["

	statusString += pr.MergeStatus.ChecksPass.String(config)

	if config.RequireApproval {
		if pr.MergeStatus.ReviewApproved {
			statusString += checkmark
		} else {
			statusString += crossmark
		}
	} else {
		statusString += "-"
	}

	if pr.MergeStatus.NoConflicts {
		statusString += checkmark
	} else {
		statusString += crossmark
	}

	if pr.MergeStatus.Stacked {
		statusString += checkmark
	} else {
		statusString += crossmark
	}

	statusString += "]"

	return fmt.Sprintf("%s %3d: %s", statusString, pr.Number, pr.Title)
}

func (cs checkStatus) String(config *Config) string {
	if config.RequireChecks {
		switch cs {
		case checkStatusUnknown:
			return "?"
		case checkStatusPending:
			return middledot
		case checkStatusFail:
			return crossmark
		case checkStatusPass:
			return checkmark
		default:
			return "?"
		}
	}
	return "-"
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
