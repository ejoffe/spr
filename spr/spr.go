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
	"sync"

	"github.com/ejoffe/profiletimer"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

// NewStackedPR constructs and returns a new stackediff instance.
func NewStackedPR(config *config.Config, github github.GitHubInterface, gitcmd git.GitInterface, writer io.Writer) *stackediff {

	return &stackediff{
		config:       config,
		github:       github,
		gitcmd:       gitcmd,
		writer:       writer,
		profiletimer: profiletimer.StartNoopTimer(),
	}
}

type stackediff struct {
	config        *config.Config
	github        github.GitHubInterface
	gitcmd        git.GitInterface
	writer        io.Writer
	profiletimer  profiletimer.Timer
	DetailEnabled bool
}

// AmendCommit enables one to easily amend a commit in the middle of a stack
//  of commits. A list of commits is printed and one can be chosen to be amended.
func (sd *stackediff) AmendCommit(ctx context.Context) {
	localCommits := sd.getLocalCommitStack()
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
	sd.mustgit("commit --fixup "+localCommits[commitIndex].CommitHash, nil)
	sd.mustgit("rebase origin/master -i --autosquash --autostash", nil)
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
	sd.profiletimer.Step("UpdatePullRequests::FetchAndGetGitHubInfo")
	localCommits := sd.getLocalCommitStack()
	sd.profiletimer.Step("UpdatePullRequests::GetLocalCommitStack")

	reorder := false
	if commitsReordered(localCommits, githubInfo.PullRequests) {
		reorder = true
		// if commits have been reordered :
		//   first - rebase all pull requests to master
		//   then - update all pull requests
		for _, pr := range githubInfo.PullRequests {
			sd.github.UpdatePullRequest(ctx, githubInfo, pr, pr.Commit, nil)
		}
		sd.profiletimer.Step("UpdatePullRequests::ReparentPullRequestsToMaster")
	}

	sd.syncCommitStackToGitHub(ctx, localCommits, githubInfo)
	sd.profiletimer.Step("UpdatePullRequests::SyncCommitStackToGithub")

	// iterate through local_commits and update pull_requests
	for commitIndex, c := range localCommits {
		if c.WIP {
			break
		}
		prFound := false
		for _, pr := range githubInfo.PullRequests {
			if c.CommitID == pr.Commit.CommitID {
				prFound = true
				if c.CommitHash != pr.Commit.CommitHash || reorder {
					// if commit id is same but commit hash changed it means the commit
					//  has been amended and we need to update the pull request
					// in the reorder case we also want to update the pull request

					var prevCommit *git.Commit
					if commitIndex > 0 {
						prevCommit = &localCommits[commitIndex-1]
					}
					sd.github.UpdatePullRequest(ctx, githubInfo, pr, c, prevCommit)
					pr.Commit = c
				}
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
		}
	}
	sd.profiletimer.Step("UpdatePullRequests::updatePullRequests")

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

	// Update the base of the merging pr to master
	sd.github.UpdatePullRequest(ctx, githubInfo, prToMerge, prToMerge.Commit, nil)
	sd.profiletimer.Step("MergePullRequests::update pr base")

	// Merge pull request
	sd.github.MergePullRequest(ctx, prToMerge)
	sd.profiletimer.Step("MergePullRequests::merge pr")

	if sd.config.User.CleanupRemoteBranch {
		sd.gitcmd.Git(fmt.Sprintf("push -d origin %s", prToMerge.FromBranch), nil)
	}

	// Close all the pull requests in the stack below the merged pr
	//  Before closing add a review comment with the pr that merged the commit.
	for i := 0; i < prIndex; i++ {
		pr := githubInfo.PullRequests[i]
		comment := fmt.Sprintf(
			"commit MERGED in pull request [#%d](https://github.com/%s/%s/pull/%d)",
			prToMerge.Number, sd.config.Repo.GitHubRepoOwner, sd.config.Repo.GitHubRepoName, prToMerge.Number)
		sd.github.CommentPullRequest(ctx, pr, comment)

		sd.github.ClosePullRequest(ctx, pr)

		if sd.config.User.CleanupRemoteBranch {
			sd.gitcmd.Git(fmt.Sprintf("push -d origin %s", pr.FromBranch), nil)
		}
	}
	sd.profiletimer.Step("MergePullRequests::close prs")

	for i := 0; i <= prIndex; i++ {
		pr := githubInfo.PullRequests[i]
		pr.Merged = true
		fmt.Fprintf(sd.writer, "%s\n", pr.String(sd.config))
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
		fmt.Fprintf(sd.writer, "pull request stack is empty\n")
	} else {
		if sd.DetailEnabled {
			fmt.Fprint(sd.writer, detailMessage)
		}
		for i := len(githubInfo.PullRequests) - 1; i >= 0; i-- {
			pr := githubInfo.PullRequests[i]
			fmt.Fprintf(sd.writer, "%s\n", pr.String(sd.config))
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
	sd.mustgit("log origin/master..HEAD", &commitLog)
	return sd.parseLocalCommitStack(commitLog)
}

func (sd *stackediff) parseLocalCommitStack(commitLog string) []git.Commit {
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
				//  last commit is missing the commit-id
				sd.printCommitInstallHelper()
				return nil
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
		sd.printCommitInstallHelper()
		return nil
	}

	return commits
}

func commitsReordered(localCommits []git.Commit, pullRequests []*github.PullRequest) bool {
	for i := 0; i < len(pullRequests); i++ {
		if localCommits[i].CommitID != pullRequests[i].Commit.CommitID {
			return true
		}
	}
	return false
}

var commitInstallHelper = `
A commit is missing a commit-id.
This most likely means the commit-msg hook isn't installed.
To install the hook run the following cmd in the repo root dir:
> ln -s $(which spr_commit_hook) .git/hooks/commit-msg
After installing the hook, you'll need to amend your commits.
`

func (sd *stackediff) printCommitInstallHelper() {
	message := strings.TrimSpace(commitInstallHelper) + "\n"
	fmt.Fprint(sd.writer, message)
}

func (sd *stackediff) fetchAndGetGitHubInfo(ctx context.Context) *github.GitHubInfo {
	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	fetch := func() {
		sd.mustgit("fetch", nil)
		sd.mustgit("rebase origin/master --autostash", nil)
		waitgroup.Done()
	}

	go fetch()
	info := sd.github.GetInfo(ctx, sd.gitcmd)
	waitgroup.Wait()

	return info
}

// syncCommitStackToGitHub gets all the local commits in the given branch
//  which are new (on top of origin/master) and creates a corresponding
//  branch on github for each commit.
func (sd *stackediff) syncCommitStackToGitHub(ctx context.Context,
	commits []git.Commit, info *github.GitHubInfo) {

	var output string
	sd.mustgit("status --porcelain --untracked-files=no", &output)
	if output != "" {
		sd.mustgit("stash", nil)
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

	var branchNames []string
	for _, commit := range updatedCommits {
		branch := sd.branchNameFromCommit(info, commit)
		branchNames = append(branchNames, branch)
		sd.mustgit("checkout "+commit.CommitHash, nil)
		sd.mustgit("switch -C "+branch, nil)
		sd.mustgit("switch "+info.LocalBranch, nil)
		sd.profiletimer.Step("SyncCommitStack::CreateBranch::" + branch)
	}
	if len(updatedCommits) > 0 {
		sd.mustgit("push --force --atomic origin "+strings.Join(branchNames, " "), nil)
	}
	for _, commit := range updatedCommits {
		branch := sd.branchNameFromCommit(info, commit)
		sd.mustgit("branch -D "+branch, nil)
	}
	sd.profiletimer.Step("SyncCommitStack::PushBranches")
}

func (sd *stackediff) pushCommitToRemote(commit git.Commit, info *github.GitHubInfo) {
	headRefName := sd.branchNameFromCommit(info, commit)
	sd.mustgit("checkout "+commit.CommitHash, nil)
	sd.mustgit("switch -C "+headRefName, nil)
	sd.mustgit("push --force --set-upstream origin "+headRefName, nil)
	sd.mustgit("switch "+info.LocalBranch, nil)
	sd.mustgit("branch -D "+headRefName, nil)
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
		panic(err)
	}
}

var detailMessage = `
 ┌─ github checks pass
 │┌── pull request approved
 ││┌─── no merge conflicts
 │││┌──── stack check
 ││││
`
