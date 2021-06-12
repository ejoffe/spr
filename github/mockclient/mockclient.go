package mockclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/stretchr/testify/require"
)

func NewMockClient(t *testing.T) *mockclient {
	return &mockclient{
		assert: require.New(t),
	}
}

type mockclient struct {
	assert *require.Assertions
	Info   *github.GitHubInfo
	expect []expectation
}

func (c *mockclient) GetInfo(ctx context.Context, gitcmd git.Cmd) *github.GitHubInfo {
	fmt.Printf("HUB: GetInfo\n")
	c.verifyExpectation(expectation{
		op: getInfoOP,
	})
	return c.Info
}

func (c *mockclient) CreatePullRequest(ctx context.Context, info *github.GitHubInfo,
	commit git.Commit, prevCommit *git.Commit) *github.PullRequest {
	fmt.Printf("HUB: CreatePullRequest\n")
	c.verifyExpectation(expectation{
		op:     createPullRequestOP,
		commit: commit,
		prev:   prevCommit,
	})

	// TODO - don't hardcode ID and Number
	// TODO - set FromBranch and ToBranch correctly
	return &github.PullRequest{
		ID:         "001",
		Number:     1,
		FromBranch: "from_branch",
		ToBranch:   "to_branch",
		Commit:     commit,
		Title:      commit.Subject,
		MergeStatus: github.PullRequestMergeStatus{
			ChecksPass:     github.CheckStatusPass,
			ReviewApproved: true,
			NoConflicts:    true,
			Stacked:        true,
		},
	}
}

func (c *mockclient) UpdatePullRequest(ctx context.Context, info *github.GitHubInfo,
	pr *github.PullRequest, commit git.Commit, prevCommit *git.Commit) {
	fmt.Printf("HUB: UpdatePullRequest\n")
	c.verifyExpectation(expectation{
		op:     updatePullRequestOP,
		commit: commit,
		prev:   prevCommit,
	})
}

func (c *mockclient) CommentPullRequest(ctx context.Context, pr *github.PullRequest, comment string) {
	fmt.Printf("HUB: CommentPullRequest\n")
	c.verifyExpectation(expectation{
		op:     commentPullRequestOP,
		commit: pr.Commit,
	})
}

func (c *mockclient) MergePullRequest(ctx context.Context, pr *github.PullRequest) {
	fmt.Printf("HUB: MergePullRequest\n")
	c.verifyExpectation(expectation{
		op:     mergePullRequestOP,
		commit: pr.Commit,
	})
}

func (c *mockclient) ClosePullRequest(ctx context.Context, pr *github.PullRequest) {
	fmt.Printf("HUB: ClosePullRequest\n")
	c.verifyExpectation(expectation{
		op:     closePullRequestOP,
		commit: pr.Commit,
	})
}

func (c *mockclient) ExpectGetInfo() {
	c.expect = append(c.expect, expectation{
		op: getInfoOP,
	})
}

func (c *mockclient) ExpectCreatePullRequest(commit git.Commit, prev *git.Commit) {
	c.expect = append(c.expect, expectation{
		op:     createPullRequestOP,
		commit: commit,
		prev:   prev,
	})
}

func (c *mockclient) ExpectUpdatePullRequest(commit git.Commit, prev *git.Commit) {
	c.expect = append(c.expect, expectation{
		op:     updatePullRequestOP,
		commit: commit,
		prev:   prev,
	})
}

func (c *mockclient) ExpectCommentPullRequest(commit git.Commit) {
	c.expect = append(c.expect, expectation{
		op:     commentPullRequestOP,
		commit: commit,
	})
}

func (c *mockclient) ExpectMergePullRequest(commit git.Commit) {
	c.expect = append(c.expect, expectation{
		op:     mergePullRequestOP,
		commit: commit,
	})
}

func (c *mockclient) ExpectClosePullRequest(commit git.Commit) {
	c.expect = append(c.expect, expectation{
		op:     closePullRequestOP,
		commit: commit,
	})
}

func (c *mockclient) verifyExpectation(actual expectation) {
	expected := c.expect[0]
	c.assert.Equal(expected, actual)
	c.expect = c.expect[1:]
}

type operation string

const (
	getInfoOP            operation = "GetInfo"
	createPullRequestOP  operation = "CreatePullRequest"
	updatePullRequestOP  operation = "UpdatePullRequest"
	commentPullRequestOP operation = "CommentPullRequest"
	mergePullRequestOP   operation = "MergePullRequest"
	closePullRequestOP   operation = "ClosePullRequest"
)

type expectation struct {
	op     operation
	commit git.Commit
	prev   *git.Commit
}
