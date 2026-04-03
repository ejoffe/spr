package spr

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/ejoffe/spr/github/mockclient"
	"github.com/stretchr/testify/require"
)

func makeTestObjects(t *testing.T, synchronized bool) (
	s *stackediff, gitmock *mockgit.Mock, githubmock *mockclient.MockClient,
	input *bytes.Buffer, output *bytes.Buffer) {
	cfg := config.EmptyConfig()
	cfg.Repo.RequireChecks = true
	cfg.Repo.RequireApproval = true
	cfg.Repo.GitHubRemote = "origin"
	cfg.Repo.GitHubBranch = "master"
	cfg.Repo.MergeMethod = "rebase"
	gitmock = mockgit.NewMockGit(t)
	githubmock = mockclient.NewMockClient(t)
	githubmock.Info = &github.GitHubInfo{
		UserName:     "TestSPR",
		RepositoryID: "RepoID",
		LocalBranch:  "master",
	}
	s = NewStackedPR(cfg, githubmock, gitmock)
	output = &bytes.Buffer{}
	s.output = output
	input = &bytes.Buffer{}
	s.input = input
	s.synchronized = synchronized
	githubmock.Synchronized = synchronized
	return
}

func TestSPRBasicFlowFourCommitsQueue(t *testing.T) {
	testSPRBasicFlowFourCommitsQueue(t, true)
	testSPRBasicFlowFourCommitsQueue(t, false)
}

func testSPRBasicFlowFourCommitsQueue(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr status' :: StatusPullRequest
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		assert.Equal("pull request stack is empty\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("[vvvv]   1 : test commit 1\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines := strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("warning: not updating reviewers for PR #1", lines[0])
		assert.Equal("[vvvv]   1 : test commit 2", lines[1])
		assert.Equal("[vvvv]   1 : test commit 1", lines[2])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c3, &c4})

		// For the first "create" call we should call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		// For the first "create" call we should *not* call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines = strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal([]string{
			"warning: not updating reviewers for PR #1",
			"warning: not updating reviewers for PR #1",
			"[vvvv]   1 : test commit 4",
			"[vvvv]   1 : test commit 3",
			"[vvvv]   1 : test commit 2",
			"[vvvv]   1 : test commit 1",
		}, lines[:6])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2]
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		count := uint(2)
		s.MergePullRequests(ctx, &count)
		lines = strings.Split(output.String(), "\n")
		assert.Equal("MERGED   1 : test commit 1", lines[0])
		assert.Equal("MERGED   1 : test commit 2", lines[1])
		fmt.Printf("OUT: %s\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		githubmock.Info.PullRequests = githubmock.Info.PullRequests[1:]
		githubmock.Info.PullRequests[0].Merged = false
		githubmock.Info.PullRequests[0].Commits = append(githubmock.Info.PullRequests[0].Commits, c1, c2)
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		githubmock.ExpectGetInfo()

		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectStatus()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})

		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines = strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal([]string{
			"warning: not updating reviewers for PR #1",
			"warning: not updating reviewers for PR #1",
			"warning: not updating reviewers for PR #1",
			"[vvvv]   1 : test commit 4",
			"[vvvv]   1 : test commit 3",
			"[vvvv] !   1 : test commit 2",
		}, lines[:6])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr merge' :: MergePullRequest :: commits=[a2, a3, a4]
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c4, nil)
		githubmock.ExpectMergePullRequest(c4, genclient.PullRequestMergeMethod_REBASE)

		githubmock.ExpectCommentPullRequest(c2)
		githubmock.ExpectClosePullRequest(c2)
		githubmock.ExpectCommentPullRequest(c3)
		githubmock.ExpectClosePullRequest(c3)

		githubmock.Info.PullRequests[0].InQueue = true

		s.MergePullRequests(ctx, nil)
		lines = strings.Split(output.String(), "\n")
		assert.Equal("MERGED .   1 : test commit 2", lines[0])
		assert.Equal("MERGED   1 : test commit 3", lines[1])
		assert.Equal("MERGED   1 : test commit 4", lines[2])
		fmt.Printf("OUT: %s\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()
	})
}

func TestSPRBasicFlowFourCommits(t *testing.T) {
	testSPRBasicFlowFourCommits(t, true)
	testSPRBasicFlowFourCommits(t, false)
}

func testSPRBasicFlowFourCommits(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr status' :: StatusPullRequest
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		assert.Equal("pull request stack is empty\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("[vvvv]   1 : test commit 1\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines := strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("warning: not updating reviewers for PR #1", lines[0])
		assert.Equal("[vvvv]   1 : test commit 2", lines[1])
		assert.Equal("[vvvv]   1 : test commit 1", lines[2])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c3, &c4})

		// For the first "create" call we should call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		// For the first "create" call we should *not* call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})

		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines = strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal([]string{
			"warning: not updating reviewers for PR #1",
			"warning: not updating reviewers for PR #1",
			"[vvvv]   1 : test commit 4",
			"[vvvv]   1 : test commit 3",
			"[vvvv]   1 : test commit 2",
			"[vvvv]   1 : test commit 1",
		}, lines[:6])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2, a3, a4]
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c4, nil)
		githubmock.ExpectMergePullRequest(c4, genclient.PullRequestMergeMethod_REBASE)
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		githubmock.ExpectCommentPullRequest(c2)
		githubmock.ExpectClosePullRequest(c2)
		githubmock.ExpectCommentPullRequest(c3)
		githubmock.ExpectClosePullRequest(c3)
		s.MergePullRequests(ctx, nil)
		lines = strings.Split(output.String(), "\n")
		assert.Equal("MERGED   1 : test commit 1", lines[0])
		assert.Equal("MERGED   1 : test commit 2", lines[1])
		assert.Equal("MERGED   1 : test commit 3", lines[2])
		assert.Equal("MERGED   1 : test commit 4", lines[3])
		fmt.Printf("OUT: %s\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()
	})
}

func TestSPRBasicFlowDeleteBranch(t *testing.T) {
	testSPRBasicFlowDeleteBranch(t, true)
	testSPRBasicFlowDeleteBranch(t, false)
}

func testSPRBasicFlowDeleteBranch(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		s.config.User.DeleteMergedBranches = true
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}

		// 'git spr update' :: UpdatePullRequest :: commits=[c1]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("[vvvv]   1 : test commit 1\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines := strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("warning: not updating reviewers for PR #1", lines[0])
		assert.Equal("[vvvv]   1 : test commit 2", lines[1])
		assert.Equal("[vvvv]   1 : test commit 1", lines[2])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2]
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		gitmock.ExpectDeleteBranch("from_branch") // <--- This is the key expectation of this test.
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		gitmock.ExpectDeleteBranch("from_branch") // <--- This is the key expectation of this test.
		s.MergePullRequests(ctx, nil)
		lines = strings.Split(output.String(), "\n")
		assert.Equal("MERGED   1 : test commit 1", lines[0])
		fmt.Printf("OUT: %s\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()
	})
}

func TestSPRMergeCount(t *testing.T) {
	testSPRMergeCount(t, true)
	testSPRMergeCount(t, false)
}

func testSPRMergeCount(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
		// For the first "create" call we should call GetAssignableUsers
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectGetAssignableUsers()
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectAddReviewers([]string{mockclient.NobodyUserID})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, []string{mockclient.NobodyLogin}, nil)
		lines := strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal([]string{
			"[vvvv]   1 : test commit 4",
			"[vvvv]   1 : test commit 3",
			"[vvvv]   1 : test commit 2",
			"[vvvv]   1 : test commit 1",
		}, lines[:4])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr merge --count 2' :: MergePullRequest :: commits=[a1, a2, a3, a4]
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		s.MergePullRequests(ctx, uintptr(2))
		lines = strings.Split(output.String(), "\n")
		assert.Equal("MERGED   1 : test commit 1", lines[0])
		assert.Equal("MERGED   1 : test commit 2", lines[1])
		fmt.Printf("OUT: %s\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()
	})
}

func TestSPRAmendCommit(t *testing.T) {
	testSPRAmendCommit(t, true)
	testSPRAmendCommit(t, false)
}

func testSPRAmendCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}

		// 'git spr state' :: StatusPullRequest
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		assert.Equal("pull request stack is empty\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		fmt.Printf("OUT: %s\n", output.String())
		lines := strings.Split(output.String(), "\n")
		assert.Equal("[vvvv]   1 : test commit 2", lines[0])
		assert.Equal("[vvvv]   1 : test commit 1", lines[1])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// amend commit c2
		c2.CommitHash = "c201000000000000000000000000000000000000"
		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c2})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		lines = strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("[vvvv]   1 : test commit 2", lines[0])
		assert.Equal("[vvvv]   1 : test commit 1", lines[1])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// amend commit c1
		c1.CommitHash = "c101000000000000000000000000000000000000"
		c2.CommitHash = "c202000000000000000000000000000000000000"
		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		lines = strings.Split(output.String(), "\n")
		fmt.Printf("OUT: %s\n", output.String())
		assert.Equal("[vvvv]   1 : test commit 2", lines[0])
		assert.Equal("[vvvv]   1 : test commit 1", lines[1])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr merge' :: MergePullRequest :: commits=[a1, a2]
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectMergePullRequest(c2, genclient.PullRequestMergeMethod_REBASE)
		githubmock.ExpectCommentPullRequest(c1)
		githubmock.ExpectClosePullRequest(c1)
		s.MergePullRequests(ctx, nil)
		lines = strings.Split(output.String(), "\n")
		assert.Equal("MERGED   1 : test commit 1", lines[0])
		assert.Equal("MERGED   1 : test commit 2", lines[1])
		fmt.Printf("OUT: %s\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()
	})
}

func TestSPRReorderCommit(t *testing.T) {
	testSPRReorderCommit(t, true)
	testSPRReorderCommit(t, false)
}

func testSPRReorderCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}
		c5 := git.Commit{
			CommitID:   "00000005",
			CommitHash: "c500000000000000000000000000000000000000",
			Subject:    "test commit 5",
		}

		// 'git spr status' :: StatusPullRequest
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		assert.Equal("pull request stack is empty\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		fmt.Printf("OUT: %s\n", output.String())
		lines := strings.Split(output.String(), "\n")
		assert.Equal("[vvvv]   1 : test commit 4", lines[0])
		assert.Equal("[vvvv]   1 : test commit 3", lines[1])
		assert.Equal("[vvvv]   1 : test commit 2", lines[2])
		assert.Equal("[vvvv]   1 : test commit 1", lines[3])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c2, c4, c1, c3]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c3, &c1, &c4, &c2})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c3, &c1, &c4, &c2})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c3, nil)
		githubmock.ExpectUpdatePullRequest(c4, nil)
		// reorder commits
		c1.CommitHash = "c101000000000000000000000000000000000000"
		c2.CommitHash = "c201000000000000000000000000000000000000"
		c3.CommitHash = "c301000000000000000000000000000000000000"
		c4.CommitHash = "c401000000000000000000000000000000000000"
		gitmock.ExpectPushCommits([]*git.Commit{&c2, &c4, &c1, &c3})
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c4, &c2)
		githubmock.ExpectUpdatePullRequest(c1, &c4)
		githubmock.ExpectUpdatePullRequest(c3, &c1)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		fmt.Printf("OUT: %s\n", output.String())
		// TODO : Need to update pull requests in GetInfo expect to get this check to work
		// lines = strings.Split(output.String(), "\n")
		//assert.Equal("[vvvv]   1 : test commit 3", lines[0])
		//assert.Equal("[vvvv]   1 : test commit 1", lines[1])
		//assert.Equal("[vvvv]   1 : test commit 4", lines[2])
		//assert.Equal("[vvvv]   1 : test commit 2", lines[3])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c5, c1, c2, c3, c4]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1, &c2, &c3, &c4, &c5})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1, &c2, &c3, &c4, &c5})
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, nil)
		githubmock.ExpectUpdatePullRequest(c3, nil)
		githubmock.ExpectUpdatePullRequest(c4, nil)
		// reorder commits
		c1.CommitHash = "c102000000000000000000000000000000000000"
		c2.CommitHash = "c202000000000000000000000000000000000000"
		c3.CommitHash = "c302000000000000000000000000000000000000"
		c4.CommitHash = "c402000000000000000000000000000000000000"
		gitmock.ExpectPushCommits([]*git.Commit{&c5, &c4, &c3, &c2, &c1})
		githubmock.ExpectCreatePullRequest(c5, nil)
		githubmock.ExpectUpdatePullRequest(c5, nil)
		githubmock.ExpectUpdatePullRequest(c4, &c5)
		githubmock.ExpectUpdatePullRequest(c3, &c4)
		githubmock.ExpectUpdatePullRequest(c2, &c3)
		githubmock.ExpectUpdatePullRequest(c1, &c2)
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		fmt.Printf("OUT: %s\n", output.String())
		// TODO : Need to update pull requests in GetInfo expect to get this check to work
		// lines = strings.Split(output.String(), "\n")
		//assert.Equal("[vvvv]   1 : test commit 5", lines[0])
		//assert.Equal("[vvvv]   1 : test commit 4", lines[1])
		//assert.Equal("[vvvv]   1 : test commit 3", lines[2])
		//assert.Equal("[vvvv]   1 : test commit 2", lines[3])
		//assert.Equal("[vvvv]   1 : test commit 1", lines[4])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// TODO : add a call to merge and check merge order
	})
}

func TestSPRDeleteCommit(t *testing.T) {
	testSPRDeleteCommit(t, true)
	testSPRDeleteCommit(t, false)
}

func testSPRDeleteCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, githubmock, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		c3 := git.Commit{
			CommitID:   "00000003",
			CommitHash: "c300000000000000000000000000000000000000",
			Subject:    "test commit 3",
		}
		c4 := git.Commit{
			CommitID:   "00000004",
			CommitHash: "c400000000000000000000000000000000000000",
			Subject:    "test commit 4",
		}

		// 'git spr status' :: StatusPullRequest
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.StatusPullRequests(ctx)
		assert.Equal("pull request stack is empty\n", output.String())
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
		githubmock.ExpectCreatePullRequest(c1, nil)
		githubmock.ExpectCreatePullRequest(c2, &c1)
		githubmock.ExpectCreatePullRequest(c3, &c2)
		githubmock.ExpectCreatePullRequest(c4, &c3)
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c2, &c1)
		githubmock.ExpectUpdatePullRequest(c3, &c2)
		githubmock.ExpectUpdatePullRequest(c4, &c3)
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
		githubmock.ExpectGetInfo()

		s.UpdatePullRequests(ctx, nil, nil)
		fmt.Printf("OUT: %s\n", output.String())
		lines := strings.Split(output.String(), "\n")
		assert.Equal("[vvvv]   1 : test commit 4", lines[0])
		assert.Equal("[vvvv]   1 : test commit 3", lines[1])
		assert.Equal("[vvvv]   1 : test commit 2", lines[2])
		assert.Equal("[vvvv]   1 : test commit 1", lines[3])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// 'git spr update' :: UpdatePullRequest :: commits=[c2, c4, c1, c3]
		githubmock.ExpectGetInfo()
		gitmock.ExpectFetch()
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c1})
		gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c1})
		githubmock.ExpectCommentPullRequest(c2)
		githubmock.ExpectClosePullRequest(c2)
		githubmock.ExpectCommentPullRequest(c3)
		githubmock.ExpectClosePullRequest(c3)
		// update commits
		c1.CommitHash = "c101000000000000000000000000000000000000"
		c4.CommitHash = "c401000000000000000000000000000000000000"
		githubmock.ExpectUpdatePullRequest(c1, nil)
		githubmock.ExpectUpdatePullRequest(c4, &c1)
		gitmock.ExpectPushCommits([]*git.Commit{&c1, &c4})
		gitmock.ExpectLogAndRespond([]*git.Commit{})
		githubmock.ExpectGetInfo()
		s.UpdatePullRequests(ctx, nil, nil)
		fmt.Printf("OUT: %s\n", output.String())
		// TODO : Need to update pull requests in GetInfo expect to get this check to work
		// lines = strings.Split(output.String(), "\n")
		//assert.Equal("[vvvv]   1 : test commit 3", lines[0])
		//assert.Equal("[vvvv]   1 : test commit 1", lines[1])
		//assert.Equal("[vvvv]   1 : test commit 4", lines[2])
		//assert.Equal("[vvvv]   1 : test commit 2", lines[3])
		gitmock.ExpectationsMet()
		githubmock.ExpectationsMet()
		output.Reset()

		// TODO : add a call to merge and check merge order
	})
}

func TestAmendNoCommits(t *testing.T) {
	testAmendNoCommits(t, true)
	testAmendNoCommits(t, false)
}

func testAmendNoCommits(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, _, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		gitmock.ExpectLogAndRespond([]*git.Commit{})
		s.AmendCommit(ctx)
		assert.Equal("No commits to amend\n", output.String())
	})
}

func TestAmendOneCommit(t *testing.T) {
	testAmendOneCommit(t, true)
	testAmendOneCommit(t, false)
}

func testAmendOneCommit(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, input, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		gitmock.ExpectFixup(c1.CommitHash)
		input.WriteString("1")
		s.AmendCommit(ctx)
		assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend (1): ", output.String())
	})
}

func TestAmendTwoCommits(t *testing.T) {
	testAmendTwoCommits(t, true)
	testAmendTwoCommits(t, false)
}

func testAmendTwoCommits(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, input, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}
		c2 := git.Commit{
			CommitID:   "00000002",
			CommitHash: "c200000000000000000000000000000000000000",
			Subject:    "test commit 2",
		}
		gitmock.ExpectLogAndRespond([]*git.Commit{&c1, &c2})
		gitmock.ExpectFixup(c2.CommitHash)
		input.WriteString("1")
		s.AmendCommit(ctx)
		assert.Equal(" 2 : 00000001 : test commit 1\n 1 : 00000002 : test commit 2\nCommit to amend (1-2): ", output.String())
	})
}

func TestAmendInvalidInput(t *testing.T) {
	testAmendInvalidInput(t, true)
	testAmendInvalidInput(t, false)
}

func testAmendInvalidInput(t *testing.T, sync bool) {
	t.Run(fmt.Sprintf("Sync: %v", sync), func(t *testing.T) {
		s, gitmock, _, input, output := makeTestObjects(t, sync)
		assert := require.New(t)
		ctx := context.Background()

		c1 := git.Commit{
			CommitID:   "00000001",
			CommitHash: "c100000000000000000000000000000000000000",
			Subject:    "test commit 1",
		}

		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		input.WriteString("a")
		s.AmendCommit(ctx)
		assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend (1): Invalid input\n", output.String())
		gitmock.ExpectationsMet()
		output.Reset()

		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		input.WriteString("0")
		s.AmendCommit(ctx)
		assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend (1): Invalid input\n", output.String())
		gitmock.ExpectationsMet()
		output.Reset()

		gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
		input.WriteString("2")
		s.AmendCommit(ctx)
		assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend (1): Invalid input\n", output.String())
		gitmock.ExpectationsMet()
		output.Reset()
	})
}

func uintptr(a uint) *uint {
	return &a
}

// --- Edit command name tests ---

// stubVcsOps is a minimal VCSOperations stub for testing user-facing messages.
type stubVcsOps struct {
	editing     bool
	commandName string
	commits     []git.Commit
}

func (s *stubVcsOps) FetchAndRebase(cfg *config.Config) error                              { return nil }
func (s *stubVcsOps) GetLocalCommitStack(cfg *config.Config, gitcmd git.GitInterface) []git.Commit {
	return s.commits
}
func (s *stubVcsOps) AmendInto(commit git.Commit) error          { return nil }
func (s *stubVcsOps) EditStart(commit git.Commit) error          { return nil }
func (s *stubVcsOps) EditFinish() error                          { return nil }
func (s *stubVcsOps) EditAbort() error                           { return nil }
func (s *stubVcsOps) PrepareForPush() (func(), error)            { return func() {}, nil }
func (s *stubVcsOps) PushBranches(cfg *config.Config, commits []git.Commit, individually bool) error {
	return nil
}
func (s *stubVcsOps) IsEditing() bool            { return s.editing }
func (s *stubVcsOps) EditStatePath() string       { return "" }
func (s *stubVcsOps) CheckStackCompleteness() string { return "" }
func (s *stubVcsOps) CommandName() string         { return s.commandName }

func TestEditCommit_AlreadyEditing_UsesCommandName(t *testing.T) {
	tests := []struct {
		name        string
		commandName string
		wantPrefix  string
	}{
		{"jj mode", "jj spr", "jj spr"},
		{"git mode", "git spr", "git spr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.EmptyConfig()
			cfg.Repo.GitHubBranch = "master"
			gitmock := mockgit.NewMockGit(t)
			githubmock := mockclient.NewMockClient(t)
			githubmock.Info = &github.GitHubInfo{UserName: "test", RepositoryID: "repo", LocalBranch: "master"}

			stub := &stubVcsOps{editing: true, commandName: tt.commandName}
			s := NewStackedPR(cfg, githubmock, gitmock, stub)
			output := &bytes.Buffer{}
			s.output = output

			s.EditCommit(context.Background())

			require.Contains(t, output.String(), tt.wantPrefix+" edit --done")
			require.Contains(t, output.String(), tt.wantPrefix+" edit --abort")
		})
	}
}

func TestEditCommit_Success_UsesCommandName(t *testing.T) {
	cfg := config.EmptyConfig()
	cfg.Repo.GitHubBranch = "master"
	gitmock := mockgit.NewMockGit(t)
	githubmock := mockclient.NewMockClient(t)
	githubmock.Info = &github.GitHubInfo{UserName: "test", RepositoryID: "repo", LocalBranch: "master"}

	stub := &stubVcsOps{
		editing:     false,
		commandName: "jj spr",
		commits: []git.Commit{
			{CommitID: "00000001", CommitHash: "c100000000000000000000000000000000000000", Subject: "test commit"},
		},
	}
	s := NewStackedPR(cfg, githubmock, gitmock, stub)
	output := &bytes.Buffer{}
	s.output = output
	input := bytes.NewBufferString("1\n")
	s.input = input

	s.EditCommit(context.Background())

	require.Contains(t, output.String(), "jj spr edit --done")
	require.Contains(t, output.String(), "jj spr edit --abort")
	require.NotContains(t, output.String(), "git spr")
}
