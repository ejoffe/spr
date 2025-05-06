package spr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/ejoffe/profiletimer"
	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/config/config_parser"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
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

	output       io.Writer
	input        io.Reader
	synchronized bool // When true code is executed without goroutines. Allows test to be deterministic
}

// AmendCommit enables one to easily amend a commit in the middle of a stack
//
//	of commits. A list of commits is printed and one can be chosen to be amended.
func (sd *stackediff) AmendCommit(ctx context.Context) {
	localCommits := git.GetLocalCommitStack(sd.config, sd.gitcmd)
	if len(localCommits) == 0 {
		fmt.Fprintf(sd.output, "No commits to amend\n")
		return
	}

	for i := len(localCommits) - 1; i >= 0; i-- {
		commit := localCommits[i]
		fmt.Fprintf(sd.output, " %d : %s : %s\n", i+1, commit.CommitID[0:8], commit.Subject)
	}

	if len(localCommits) == 1 {
		fmt.Fprintf(sd.output, "Commit to amend (%d): ", 1)
	} else {
		fmt.Fprintf(sd.output, "Commit to amend (%d-%d): ", 1, len(localCommits))
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
	sd.gitcmd.MustGit("commit --fixup "+localCommits[commitIndex].CommitHash, nil)

	rebaseCmd := fmt.Sprintf("rebase -i --autosquash --autostash %s/%s",
		sd.config.Repo.GitHubRemote, sd.config.Repo.GitHubBranch)
	sd.gitcmd.MustGit(rebaseCmd, nil)
}

func (sd *stackediff) addReviewers(ctx context.Context,
	pr *github.PullRequest, reviewers []string, assignable []github.RepoAssignee) {
	userIDs := make([]string, 0, len(reviewers))
	for _, r := range reviewers {
		found := false
		for _, u := range assignable {
			if strings.EqualFold(r, u.Login) {
				found = true
				userIDs = append(userIDs, u.ID)
				break
			}
		}
		if !found {
			check(fmt.Errorf("unable to add reviewer, user %q not found", r))
		}
	}
	sd.github.AddReviewers(ctx, pr, userIDs)
}

func alignLocalCommits(commits []git.Commit, prs []*github.PullRequest) []git.Commit {
	var remoteCommits = map[string]bool{}
	for _, pr := range prs {
		for _, c := range pr.Commits {
			remoteCommits[c.CommitID] = c.CommitID == pr.Commit.CommitID
		}
	}

	var result []git.Commit
	for _, commit := range commits {
		if head, ok := remoteCommits[commit.CommitID]; ok && !head {
			continue
		}

		result = append(result, commit)
	}

	return result
}

// UpdatePullRequests implements a stacked diff workflow on top of github.
//
//	Each time it's called it compares the local branch unmerged commits
//	 with currently open pull requests in github.
//	It will create a new pull request for all new commits, and update the
//	 pull request if a commit has been amended.
//	In the case where commits are reordered, the corresponding pull requests
//	 will also be reordered to match the commit stack order.
func (sd *stackediff) UpdatePullRequests(ctx context.Context, reviewers []string, count *uint) {
	sd.profiletimer.Step("UpdatePullRequests::Start")
	githubInfo := sd.fetchAndGetGitHubInfo(ctx)
	if githubInfo == nil {
		return
	}
	sd.profiletimer.Step("UpdatePullRequests::FetchAndGetGitHubInfo")
	localCommits := alignLocalCommits(git.GetLocalCommitStack(sd.config, sd.gitcmd), githubInfo.PullRequests)
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
		wg := new(sync.WaitGroup)
		wg.Add(len(githubInfo.PullRequests))

		// if commits have been reordered :
		//   first - rebase all pull requests to target branch
		//   then - update all pull requests
		for i := range githubInfo.PullRequests {
			fn := func(i int) {
				pr := githubInfo.PullRequests[i]
				sd.github.UpdatePullRequest(ctx, sd.gitcmd, githubInfo.PullRequests, pr, pr.Commit, nil)
				wg.Done()
			}
			if sd.synchronized {
				fn(i)
			} else {
				go fn(i)
			}
		}

		wg.Wait()
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
	var assignable []github.RepoAssignee

	// iterate through local_commits and update pull_requests
	var prevCommit *git.Commit
	for commitIndex, c := range localCommits {
		if c.WIP {
			break
		}
		prFound := false
		for _, pr := range githubInfo.PullRequests {
			if c.CommitID == pr.Commit.CommitID {
				prFound = true
				updateQueue = append(updateQueue, prUpdate{pr, c, prevCommit})
				pr.Commit = c
				if len(reviewers) != 0 {
					fmt.Fprintf(sd.output, "warning: not updating reviewers for PR #%d\n", pr.Number)
				}
				prevCommit = &localCommits[commitIndex]
				break
			}
		}
		if !prFound {
			// if pull request is not found for this commit_id it means the commit
			//  is new and we need to create a new pull request
			pr := sd.github.CreatePullRequest(ctx, sd.gitcmd, githubInfo, c, prevCommit)
			githubInfo.PullRequests = append(githubInfo.PullRequests, pr)
			updateQueue = append(updateQueue, prUpdate{pr, c, prevCommit})
			if len(reviewers) != 0 {
				if assignable == nil {
					assignable = sd.github.GetAssignableUsers(ctx)
				}
				sd.addReviewers(ctx, pr, reviewers, assignable)
			}
			prevCommit = &localCommits[commitIndex]
		}

		if count != nil && (commitIndex+1) == int(*count) {
			break
		}
	}
	sd.profiletimer.Step("UpdatePullRequests::updatePullRequests")

	wg := new(sync.WaitGroup)
	wg.Add(len(updateQueue))

	// Sort the PR stack by the local commit order, in case some commits were reordered
	sortedPullRequests := sortPullRequestsByLocalCommitOrder(githubInfo.PullRequests, localCommits)
	for i := range updateQueue {
		fn := func(i int) {
			pr := updateQueue[i]
			sd.github.UpdatePullRequest(ctx, sd.gitcmd, sortedPullRequests, pr.pr, pr.commit, pr.prevCommit)
			wg.Done()
		}
		if sd.synchronized {
			fn(i)
		} else {
			go fn(i)
		}
	}

	wg.Wait()

	sd.profiletimer.Step("UpdatePullRequests::commitUpdateQueue")

	sd.StatusPullRequests(ctx)
}

// MergePullRequests will go through all the current pull requests
//
//	and merge all requests that are mergeable.
//
// For a pull request to be mergeable it has to:
//   - have at least one approver
//   - pass all checks
//   - have no merge conflicts
//   - not be on top of another unmergable request
//   - pass merge checks (using 'spr check') if configured
//
// In order to merge a stack of pull requests without generating conflicts
//
//	and other pr issues. We find the top mergeable pull request in the stack,
//	than we change this pull request's base to be master and then merge the
//	pull request. This one merge in effect merges all the commits in the stack.
//	We than close all the pull requests which are below the merged request, as
//	their commits have already been merged.
func (sd *stackediff) MergePullRequests(ctx context.Context, count *uint) {
	sd.profiletimer.Step("MergePullRequests::Start")
	githubInfo := sd.github.GetInfo(ctx, sd.gitcmd)
	sd.profiletimer.Step("MergePullRequests::getGitHubInfo")

	// MergeCheck
	if sd.config.Repo.MergeCheck != "" {
		localCommits := git.GetLocalCommitStack(sd.config, sd.gitcmd)
		if len(localCommits) > 0 {
			lastCommit := localCommits[len(localCommits)-1]
			checkedCommit, found := sd.config.State.MergeCheckCommit[githubInfo.Key()]

			if !found {
				check(errors.New("need to run merge check 'spr check' before merging"))
			} else if checkedCommit != "SKIP" && lastCommit.CommitHash != checkedCommit {
				check(errors.New("need to run merge check 'spr check' before merging"))
			}
		}
	}

	// Figure out top most pr in the stack that is mergeable
	var prIndex int
	for prIndex = 0; prIndex < len(githubInfo.PullRequests); prIndex++ {
		pr := githubInfo.PullRequests[prIndex]
		if !pr.Mergeable(sd.config) {
			prIndex--
			break
		}
		if count != nil && (prIndex+1) == int(*count) {
			break
		}
	}
	if prIndex == len(githubInfo.PullRequests) {
		prIndex--
	}
	if prIndex == -1 {
		return
	}
	prToMerge := githubInfo.PullRequests[prIndex]

	// Update the base of the merging pr to target branch
	sd.github.UpdatePullRequest(ctx, sd.gitcmd, githubInfo.PullRequests, prToMerge, prToMerge.Commit, nil)
	sd.profiletimer.Step("MergePullRequests::update pr base")

	// Merge pull request
	mergeMethod, err := sd.config.MergeMethod()
	check(err)
	sd.github.MergePullRequest(ctx, prToMerge, mergeMethod)
	if sd.config.User.DeleteMergedBranches {
		sd.gitcmd.DeleteRemoteBranch(ctx, prToMerge.FromBranch)
	}

	// Close all the pull requests in the stack below the merged pr
	//  Before closing add a review comment with the pr that merged the commit.
	for i := 0; i < prIndex; i++ {
		pr := githubInfo.PullRequests[i]
		comment := fmt.Sprintf(
			"✓ Commit merged in pull request [#%d](https://%s/%s/%s/pull/%d)",
			prToMerge.Number, sd.config.Repo.GitHubHost, sd.config.Repo.GitHubRepoOwner, sd.config.Repo.GitHubRepoName, prToMerge.Number)
		sd.github.CommentPullRequest(ctx, pr, comment)
		sd.github.ClosePullRequest(ctx, pr)
		if sd.config.User.DeleteMergedBranches {
			sd.gitcmd.DeleteRemoteBranch(ctx, pr.FromBranch)
		}
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
//
//	prints out the status of each. It does not make any updates locally or
//	remotely on github.
func (sd *stackediff) StatusPullRequests(ctx context.Context) {
	sd.profiletimer.Step("StatusPullRequests::Start")
	githubInfo := sd.github.GetInfo(ctx, sd.gitcmd)

	if len(githubInfo.PullRequests) == 0 {
		fmt.Fprintf(sd.output, "pull request stack is empty\n")
	} else {
		if sd.DetailEnabled {
			fmt.Fprint(sd.output, header(sd.config))
		}
		for i := len(githubInfo.PullRequests) - 1; i >= 0; i-- {
			pr := githubInfo.PullRequests[i]
			fmt.Fprintf(sd.output, "%s\n", pr.String(sd.config))
		}
	}
	sd.profiletimer.Step("StatusPullRequests::End")
}

// SyncStack synchronizes your local stack with remote's
func (sd *stackediff) SyncStack(ctx context.Context) {
	sd.profiletimer.Step("SyncStack::Start")
	defer sd.profiletimer.Step("SyncStack::End")

	githubInfo := sd.github.GetInfo(ctx, sd.gitcmd)

	if len(githubInfo.PullRequests) == 0 {
		fmt.Fprintf(sd.output, "pull request stack is empty\n")
		return
	}

	lastPR := githubInfo.PullRequests[len(githubInfo.PullRequests)-1]
	syncCommand := fmt.Sprintf("cherry-pick ..%s", lastPR.Commit.CommitHash)
	err := sd.gitcmd.Git(syncCommand, nil)
	check(err)
}

func (sd *stackediff) RunMergeCheck(ctx context.Context) {
	sd.profiletimer.Step("RunMergeCheck::Start")
	defer sd.profiletimer.Step("RunMergeCheck::End")

	if sd.config.Repo.MergeCheck == "" {
		fmt.Println("use MergeCheck to configure a pre merge check command to run")
		return
	}

	localCommits := git.GetLocalCommitStack(sd.config, sd.gitcmd)
	if len(localCommits) == 0 {
		fmt.Println("no local commits - nothing to check")
		return
	}

	githubInfo := sd.github.GetInfo(ctx, sd.gitcmd)

	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigch)

	var cmd *exec.Cmd
	splitCmd := strings.Split(sd.config.Repo.MergeCheck, " ")
	if len(splitCmd) == 1 {
		cmd = exec.Command(splitCmd[0])
	} else {
		cmd = exec.Command(splitCmd[0], splitCmd[1:]...)
	}
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	check(err)

	go func() {
		_, ok := <-sigch
		if ok {
			err := cmd.Process.Signal(syscall.SIGKILL)
			check(err)
		}
	}()

	err = cmd.Wait()

	if err != nil {
		sd.config.State.MergeCheckCommit[githubInfo.Key()] = ""
		rake.LoadSources(sd.config.State,
			rake.YamlFileWriter(config_parser.InternalConfigFilePath()))
		fmt.Printf("MergeCheck FAILED: %s\n", err)
		return
	}

	lastCommit := localCommits[len(localCommits)-1]
	sd.config.State.MergeCheckCommit[githubInfo.Key()] = lastCommit.CommitHash
	rake.LoadSources(sd.config.State,
		rake.YamlFileWriter(config_parser.InternalConfigFilePath()))
	fmt.Println("MergeCheck PASSED")
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

func commitsReordered(localCommits []git.Commit, pullRequests []*github.PullRequest) bool {
	for i := 0; i < len(pullRequests); i++ {
		if localCommits[i].CommitID != pullRequests[i].Commit.CommitID {
			return true
		}
	}
	return false
}

func sortPullRequestsByLocalCommitOrder(pullRequests []*github.PullRequest, localCommits []git.Commit) []*github.PullRequest {
	pullRequestMap := map[string]*github.PullRequest{}
	for _, pullRequest := range pullRequests {
		pullRequestMap[pullRequest.Commit.CommitID] = pullRequest
	}

	var sortedPullRequests []*github.PullRequest
	for _, commit := range localCommits {
		if !commit.WIP && pullRequestMap[commit.CommitID] != nil {
			sortedPullRequests = append(sortedPullRequests, pullRequestMap[commit.CommitID])
		}
	}
	return sortedPullRequests
}

func (sd *stackediff) fetchAndGetGitHubInfo(ctx context.Context) *github.GitHubInfo {
	if sd.config.Repo.ForceFetchTags {
		sd.gitcmd.MustGit("fetch --tags --force", nil)
	} else {
		sd.gitcmd.MustGit("fetch", nil)
	}
	rebaseCommand := fmt.Sprintf("rebase %s/%s --autostash",
		sd.config.Repo.GitHubRemote, sd.config.Repo.GitHubBranch)
	err := sd.gitcmd.Git(rebaseCommand, nil)
	if err != nil {
		return nil
	}
	info := sd.github.GetInfo(ctx, sd.gitcmd)
	if git.BranchNameRegex.FindString(info.LocalBranch) != "" {
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
//
//	which are new (on top of remote branch) and creates a corresponding
//	branch on github for each commit.
func (sd *stackediff) syncCommitStackToGitHub(ctx context.Context,
	commits []git.Commit, info *github.GitHubInfo) bool {

	var output string
	sd.gitcmd.MustGit("status --porcelain --untracked-files=no", &output)
	if output != "" {
		err := sd.gitcmd.Git("stash", nil)
		if err != nil {
			return false
		}
		defer sd.gitcmd.MustGit("stash pop", nil)
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
		branchName := git.BranchNameFromCommit(sd.config, commit)
		refNames = append(refNames,
			commit.CommitHash+":refs/heads/"+branchName)
	}

	if len(updatedCommits) > 0 {
		if sd.config.Repo.BranchPushIndividually {
			for _, refName := range refNames {
				pushCommand := fmt.Sprintf("push --force %s %s", sd.config.Repo.GitHubRemote, refName)
				sd.gitcmd.MustGit(pushCommand, nil)
			}
		} else {
			pushCommand := fmt.Sprintf("push --force --atomic %s ", sd.config.Repo.GitHubRemote)
			pushCommand += strings.Join(refNames, " ")
			sd.gitcmd.MustGit(pushCommand, nil)
		}
	}
	sd.profiletimer.Step("SyncCommitStack::PushBranches")
	return true
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

func header(config *config.Config) string {
	if config.User.StatusBitsEmojis {
		return `
 ┌─ github checks pass
 │ ┌── pull request approved
 │ │ ┌─── no merge conflicts
 │ │ │ ┌──── stack check
 │ │ │ │
`
	} else {
		return `
 ┌─ github checks pass
 │┌── pull request approved
 ││┌─── no merge conflicts
 │││┌──── stack check
 ││││
`
	}
}
