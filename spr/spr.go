package spr

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ejoffe/profiletimer"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient"
	"github.com/ejoffe/spr/hook"
)

// NewStackedPR constructs and returns a new stackediff instance.
func NewStackedPR(config *config.Config, github github.GitHubInterface, gitcmd git.GitInterface) *stackediff {

	return &stackediff{
		config:       config,
		github:       github,
		gitcmd:       gitcmd,
		profiletimer: profiletimer.StartNoopTimer(),

		output: os.Stdout,
		input:  os.Stdin,
	}
}

type stackediff struct {
	config        *config.Config
	github        github.GitHubInterface
	gitcmd        git.GitInterface
	profiletimer  profiletimer.Timer
	DetailEnabled bool

	output io.Writer
	input  io.Reader
}

// AmendCommit enables one to easily amend a commit in the middle of a stack
//  of commits. A list of commits is printed and one can be chosen to be amended.
func (sd *stackediff) AmendCommit(ctx context.Context) {
	localCommits := sd.getLocalCommitStack()
	if len(localCommits) == 0 {
		fmt.Fprintf(sd.output, "No commits to amend\n")
		return
	}

	for i := len(localCommits) - 1; i >= 0; i-- {
		commit := localCommits[i]
		fmt.Fprintf(sd.output, " %d : %s : %s\n", i+1, commit.CommitID[0:8], commit.Subject)
	}

	if len(localCommits) == 1 {
		fmt.Fprintf(sd.output, "Commit to amend [%d]: ", 1)
	} else {
		fmt.Fprintf(sd.output, "Commit to amend [%d-%d]: ", 1, len(localCommits))
	}

	reader := bufio.NewReader(sd.input)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	commitIndex, err := strconv.Atoi(line)
	if err != nil || commitIndex < 1 || commitIndex > len(localCommits) {
		fmt.Fprint(sd.output, "Invalid input\n")
		return
	}
	commitIndex = commitIndex - 1
	check(err)
	sd.mustgit("commit --fixup "+localCommits[commitIndex].CommitHash, nil)
	sd.mustgit("rebase -i --autosquash --autostash", nil)
}

// UpdatePullRequests implements a stacked diff workflow on top of github.
//  Each time it's called it compares the local branch unmerged commits
//   with currently open pull requests in github.
//  It will create a new pull request for all new commits, and update the
//   pull request if a commit has been amended.
//  In the case where commits are reordered, the corresponding pull requests
//   will also be reordered to match the commit stack order.
func (sd *stackediff) UpdatePullRequests(ctx context.Context) {
	sd.profiletimer.Step("UpdatePullRequests::Start")
	githubInfo := sd.fetchAndGetGitHubInfo(ctx)
	if githubInfo == nil {
		return
	}
	sd.profiletimer.Step("UpdatePullRequests::FetchAndGetGitHubInfo")
	localCommits := sd.getLocalCommitStack()
	sd.profiletimer.Step("UpdatePullRequests::GetLocalCommitStack")

	// close prs for deleted commits
	var validPullRequests []*github.PullRequest
	localCommitMap := map[string]*git.Commit{}
	for _, commit := range localCommits {
		localCommitMap[commit.CommitID] = &commit
	}
	for _, pr := range githubInfo.PullRequests {
		if _, found := localCommitMap[pr.Commit.CommitID]; !found {
			sd.github.CommentPullRequest(ctx, pr, "Closing pull request: commit has gone away")
			sd.github.ClosePullRequest(ctx, pr)
		} else {
			validPullRequests = append(validPullRequests, pr)
		}
	}
	githubInfo.PullRequests = validPullRequests

	if commitsReordered(localCommits, githubInfo.PullRequests) {
		// if commits have been reordered :
		//   first - rebase all pull requests to target branch
		//   then - update all pull requests
		for _, pr := range githubInfo.PullRequests {
			sd.github.UpdatePullRequest(ctx, githubInfo, pr, pr.Commit, nil)
		}
		sd.profiletimer.Step("UpdatePullRequests::ReparentPullRequestsToMaster")
	}

	if !sd.syncCommitStackToGitHub(ctx, localCommits, githubInfo) {
		return
	}
	sd.profiletimer.Step("UpdatePullRequests::SyncCommitStackToGithub")

	type prUpdate struct {
		pr         *github.PullRequest
		commit     git.Commit
		prevCommit *git.Commit
	}

	updateQueue := make([]prUpdate, 0)

	// iterate through local_commits and update pull_requests
	for commitIndex, c := range localCommits {
		if c.WIP {
			break
		}
		prFound := false
		for _, pr := range githubInfo.PullRequests {
			if c.CommitID == pr.Commit.CommitID {
				prFound = true
				var prevCommit *git.Commit
				if commitIndex > 0 {
					prevCommit = &localCommits[commitIndex-1]
				}
				updateQueue = append(updateQueue, prUpdate{pr, c, prevCommit})
				pr.Commit = c
				break
			}
		}
		if !prFound {
			// if pull request is not found for this commit_id it means the commit
			//  is new and we need to create a new pull request
			var prevCommit *git.Commit
			if commitIndex > 0 {
				prevCommit = &localCommits[commitIndex-1]
			}
			pr := sd.github.CreatePullRequest(ctx, githubInfo, c, prevCommit)
			githubInfo.PullRequests = append(githubInfo.PullRequests, pr)
			updateQueue = append(updateQueue, prUpdate{pr, c, prevCommit})
		}
	}
	sd.profiletimer.Step("UpdatePullRequests::updatePullRequests")

	for _, pr := range updateQueue {
		sd.github.UpdatePullRequest(ctx, githubInfo, pr.pr, pr.commit, pr.prevCommit)
	}

	sd.profiletimer.Step("UpdatePullRequests::commitUpdateQueue")

	sd.StatusPullRequests(ctx)
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
//  than we change this pull request's base to be master and then merge the
//  pull request. This one merge in effect merges all the commits in the stack.
//  We than close all the pull requests which are below the merged request, as
//  their commits have already been merged.
func (sd *stackediff) MergePullRequests(ctx context.Context) {
	sd.profiletimer.Step("MergePullRequests::Start")
	githubInfo := sd.github.GetInfo(ctx, sd.gitcmd)
	sd.profiletimer.Step("MergePullRequests::getGitHubInfo")

	// Figure out top most pr in the stack that is mergeable
	var prIndex int
	for prIndex = 0; prIndex < len(githubInfo.PullRequests); prIndex++ {
		pr := githubInfo.PullRequests[prIndex]
		if !pr.Mergeable(sd.config) {
			break
		}
	}
	prIndex--
	if prIndex == -1 {
		return
	}
	prToMerge := githubInfo.PullRequests[prIndex]

	// Update the base of the merging pr to target branch
	sd.github.UpdatePullRequest(ctx, githubInfo, prToMerge, prToMerge.Commit, nil)
	sd.profiletimer.Step("MergePullRequests::update pr base")

	// Merge pull request
	sd.github.MergePullRequest(ctx, prToMerge)
	sd.profiletimer.Step("MergePullRequests::merge pr")

	// Close all the pull requests in the stack below the merged pr
	//  Before closing add a review comment with the pr that merged the commit.
	for i := 0; i < prIndex; i++ {
		pr := githubInfo.PullRequests[i]
		comment := fmt.Sprintf(
			"✓ Commit merged in pull request [#%d](https://%s/%s/%s/pull/%d)",
			prToMerge.Number, sd.config.Repo.GitHubHost, sd.config.Repo.GitHubRepoOwner, sd.config.Repo.GitHubRepoName, prToMerge.Number)
		sd.github.CommentPullRequest(ctx, pr, comment)
		sd.github.ClosePullRequest(ctx, pr)
	}
	sd.profiletimer.Step("MergePullRequests::close prs")

	for i := 0; i <= prIndex; i++ {
		pr := githubInfo.PullRequests[i]
		pr.Merged = true
		fmt.Fprintf(sd.output, "%s\n", pr.String(sd.config))
	}

	sd.profiletimer.Step("MergePullRequests::End")
}

// StatusPullRequests fetches all the users pull requests from github and
//  prints out the status of each. It does not make any updates locally or
//  remotely on github.
func (sd *stackediff) StatusPullRequests(ctx context.Context) {
	sd.profiletimer.Step("StatusPullRequests::Start")
	githubInfo := sd.github.GetInfo(ctx, sd.gitcmd)

	if len(githubInfo.PullRequests) == 0 {
		fmt.Fprintf(sd.output, "pull request stack is empty\n")
	} else {
		if sd.DetailEnabled {
			fmt.Fprint(sd.output, detailMessage)
		}
		for i := len(githubInfo.PullRequests) - 1; i >= 0; i-- {
			pr := githubInfo.PullRequests[i]
			fmt.Fprintf(sd.output, "%s\n", pr.String(sd.config))
		}
	}
	sd.profiletimer.Step("StatusPullRequests::End")
}

// ProfilingEnable enables stopwatch profiling
func (sd *stackediff) ProfilingEnable() {
	sd.profiletimer = profiletimer.StartProfileTimer()
}

// ProfilingSummary prints profiling info to stdout
func (sd *stackediff) ProfilingSummary() {
	err := sd.profiletimer.ShowResults()
	check(err)
}

// getLocalCommitStack returns a list of unmerged commits
func (sd *stackediff) getLocalCommitStack() []git.Commit {
	var commitLog string
	logCommand := fmt.Sprintf("log %s/%s..HEAD",
		sd.config.Repo.GitHubRemote, sd.config.Repo.GitHubBranch)
	sd.mustgit(logCommand, &commitLog)
	commits, valid := sd.parseLocalCommitStack(commitLog)
	if !valid {
		// if not valid - it means commit hook was not installed
		//  install commit-hook and try again
		hook.InstallCommitHook(sd.config, sd.gitcmd)
		sd.mustgit(logCommand, &commitLog)
		commits, valid = sd.parseLocalCommitStack(commitLog)
		if !valid {
			errMsg := "unable to fetch local commits\n"
			errMsg += " most likely this is an issue with missing commit-id in the commit body\n"
			errMsg += " which is caused by the commit-msg hook not being installed propertly\n"
			panic(errMsg)
		}
	}
	return commits
}

func (sd *stackediff) parseLocalCommitStack(commitLog string) ([]git.Commit, bool) {
	var commits []git.Commit

	commitHashRegex := regexp.MustCompile(`^commit ([a-f0-9]{40})`)
	commitIDRegex := regexp.MustCompile(`commit-id\:([a-f0-9]{8})`)

	// The list of commits from the command line actually starts at the
	//  most recent commit. In order to reverse the list we use a
	//  custom prepend function instead of append
	prepend := func(l []git.Commit, c git.Commit) []git.Commit {
		l = append(l, git.Commit{})
		copy(l[1:], l)
		l[0] = c
		return l
	}

	// commitScanOn is set to true when the commit hash is matched
	//  and turns false when the commit-id is matched.
	//  Commit messages always start with a hash and end with a commit-id.
	//  The commit subject and body are always between the hash the commit-id.
	commitScanOn := false

	subjectIndex := 0
	var scannedCommit git.Commit

	lines := strings.Split(commitLog, "\n")
	for index, line := range lines {

		// match commit hash : start of a new commit
		matches := commitHashRegex.FindStringSubmatch(line)
		if matches != nil {
			if commitScanOn {
				// missing the commit-id
				return nil, false
			}
			commitScanOn = true
			scannedCommit = git.Commit{
				CommitHash: matches[1],
			}
			subjectIndex = index + 4
		}

		// match commit id : last thing in the commit
		matches = commitIDRegex.FindStringSubmatch(line)
		if matches != nil {
			scannedCommit.CommitID = matches[1]
			scannedCommit.Body = strings.TrimSpace(scannedCommit.Body)

			if strings.HasPrefix(scannedCommit.Subject, "WIP") {
				scannedCommit.WIP = true
			}

			commits = prepend(commits, scannedCommit)
			commitScanOn = false
		}

		// look for subject and body
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
		// missing the commit-id
		return nil, false
	}

	return commits, true
}

func commitsReordered(localCommits []git.Commit, pullRequests []*github.PullRequest) bool {
	for i := 0; i < len(pullRequests); i++ {
		if localCommits[i].CommitID != pullRequests[i].Commit.CommitID {
			return true
		}
	}
	return false
}

func (sd *stackediff) fetchAndGetGitHubInfo(ctx context.Context) *github.GitHubInfo {
	sd.mustgit("fetch", nil)
	rebaseCommand := fmt.Sprintf("rebase %s/%s --autostash",
		sd.config.Repo.GitHubRemote, sd.config.Repo.GitHubBranch)
	err := sd.gitcmd.Git(rebaseCommand, nil)
	if err != nil {
		return nil
	}
	info := sd.github.GetInfo(ctx, sd.gitcmd)
	if githubclient.BranchNameRegex.FindString(info.LocalBranch) != "" {
		fmt.Printf("error: don't run spr in a remote pr branch\n")
		fmt.Printf(" this could lead to weird duplicate pull requests getting created\n")
		fmt.Printf(" in general there is no need to checkout remote branches used for prs\n")
		fmt.Printf(" instead use local branches and run spr update to sync your commit stack\n")
		fmt.Printf("  with your pull requests on github\n")
		fmt.Printf("branch name: %s\n", info.LocalBranch)
		return nil
	}

	return info
}

// syncCommitStackToGitHub gets all the local commits in the given branch
//  which are new (on top of remote branch) and creates a corresponding
//  branch on github for each commit.
func (sd *stackediff) syncCommitStackToGitHub(ctx context.Context,
	commits []git.Commit, info *github.GitHubInfo) bool {

	var output string
	sd.mustgit("status --porcelain --untracked-files=no", &output)
	if output != "" {
		err := sd.gitcmd.Git("stash", nil)
		if err != nil {
			return false
		}
		defer sd.mustgit("stash pop", nil)
	}

	commitUpdated := func(c git.Commit, info *github.GitHubInfo) bool {
		for _, pr := range info.PullRequests {
			if pr.Commit.CommitID == c.CommitID {
				return pr.Commit.CommitHash != c.CommitHash
			}
		}
		return true
	}

	var updatedCommits []git.Commit
	for _, commit := range commits {
		if commit.WIP {
			break
		}
		if commitUpdated(commit, info) {
			updatedCommits = append(updatedCommits, commit)
		}
	}

	var refNames []string
	for _, commit := range updatedCommits {
		branchName := sd.branchNameFromCommit(info, commit)
		refNames = append(refNames,
			commit.CommitHash+":refs/heads/"+branchName)
	}
	if len(updatedCommits) > 0 {
		pushCommand := fmt.Sprintf("push --force --atomic %s ", sd.config.Repo.GitHubRemote)
		pushCommand += strings.Join(refNames, " ")
		sd.mustgit(pushCommand, nil)
	}
	sd.profiletimer.Step("SyncCommitStack::PushBranches")
	return true
}

func (sd *stackediff) branchNameFromCommit(info *github.GitHubInfo, commit git.Commit) string {
	return "pr/" + info.UserName + "/" + info.LocalBranch + "/" + commit.CommitID
}

func (sd *stackediff) mustgit(argStr string, output *string) {
	err := sd.gitcmd.Git(argStr, output)
	check(err)
}

func check(err error) {
	if err != nil {
		if os.Getenv("SPR_DEBUG") == "1" {
			panic(err)
		}
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

var detailMessage = `
 ┌─ github checks pass
 │ ┌── pull request approved
 │ │ ┌─── no merge conflicts
 │ │ │ ┌──── stack check
 │ │ │ │
`
