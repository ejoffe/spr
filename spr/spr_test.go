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
	"github.com/ejoffe/spr/github/mockclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestObjects(t *testing.T) (
	s *stackediff, gitmock *mockgit.Mock, githubmock *mockclient.MockClient,
	input *bytes.Buffer, output *bytes.Buffer) {
	cfg := config.EmptyConfig()
	cfg.Repo.RequireChecks = true
	cfg.Repo.RequireApproval = true
	cfg.Repo.GitHubRemote = "origin"
	cfg.Repo.GitHubBranch = "master"
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
	return
}

func TestSPRBasicFlowFourCommits(t *testing.T) {
	s, gitmock, githubmock, _, output := makeTestObjects(t)
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

	// 'git spr -s' :: StatusPullRequest
	githubmock.ExpectGetInfo()
	s.StatusPullRequests(ctx)
	assert.Equal("pull request stack is empty\n", output.String())
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c1]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c1})
	githubmock.ExpectCreatePullRequest(c1, nil)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	fmt.Printf("OUT: %s\n", output.String())
	assert.Equal("[✔✔✔✔]   1 : test commit 1\n", output.String())
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c2})
	githubmock.ExpectCreatePullRequest(c2, &c1)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	lines := strings.Split(output.String(), "\n")
	fmt.Printf("OUT: %s\n", output.String())
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[1])
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c3, &c4})
	githubmock.ExpectCreatePullRequest(c3, &c2)
	githubmock.ExpectCreatePullRequest(c4, &c3)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	lines = strings.Split(output.String(), "\n")
	fmt.Printf("OUT: %s\n", output.String())
	assert.Equal("[✔✔✔✔]   1 : test commit 4", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 3", lines[1])
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[2])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[3])
	output.Reset()

	// 'git spr -m' :: MergePullRequest :: commits=[a1, a2, a3, a4]
	githubmock.ExpectGetInfo()
	githubmock.ExpectUpdatePullRequest(c4, nil)
	githubmock.ExpectMergePullRequest(c4)
	githubmock.ExpectCommentPullRequest(c1)
	githubmock.ExpectClosePullRequest(c1)
	githubmock.ExpectCommentPullRequest(c2)
	githubmock.ExpectClosePullRequest(c2)
	githubmock.ExpectCommentPullRequest(c3)
	githubmock.ExpectClosePullRequest(c3)
	s.MergePullRequests(ctx)
	lines = strings.Split(output.String(), "\n")
	assert.Equal("MERGED   1 : test commit 1", lines[0])
	assert.Equal("MERGED   1 : test commit 2", lines[1])
	assert.Equal("MERGED   1 : test commit 3", lines[2])
	assert.Equal("MERGED   1 : test commit 4", lines[3])
	fmt.Printf("OUT: %s\n", output.String())
	output.Reset()
}

func TestSPRAmendCommit(t *testing.T) {
	s, gitmock, githubmock, _, output := makeTestObjects(t)
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

	// 'git spr -s' :: StatusPullRequest
	githubmock.ExpectGetInfo()
	s.StatusPullRequests(ctx)
	assert.Equal("pull request stack is empty\n", output.String())
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2})
	githubmock.ExpectCreatePullRequest(c1, nil)
	githubmock.ExpectCreatePullRequest(c2, &c1)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	fmt.Printf("OUT: %s\n", output.String())
	lines := strings.Split(output.String(), "\n")
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[1])
	output.Reset()

	// amend commit c2
	c2.CommitHash = "c201000000000000000000000000000000000000"
	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c2})
	githubmock.ExpectUpdatePullRequest(c2, &c1)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	lines = strings.Split(output.String(), "\n")
	fmt.Printf("OUT: %s\n", output.String())
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[1])
	output.Reset()

	// amend commit c1
	c1.CommitHash = "c101000000000000000000000000000000000000"
	c2.CommitHash = "c202000000000000000000000000000000000000"
	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2})
	githubmock.ExpectUpdatePullRequest(c1, nil)
	githubmock.ExpectUpdatePullRequest(c2, &c1)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	lines = strings.Split(output.String(), "\n")
	fmt.Printf("OUT: %s\n", output.String())
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[1])
	output.Reset()

	// 'git spr -m' :: MergePullRequest :: commits=[a1, a2]
	githubmock.ExpectGetInfo()
	githubmock.ExpectUpdatePullRequest(c2, nil)
	githubmock.ExpectMergePullRequest(c2)
	githubmock.ExpectCommentPullRequest(c1)
	githubmock.ExpectClosePullRequest(c1)
	githubmock.ExpectCommentPullRequest(c2)
	githubmock.ExpectClosePullRequest(c2)
	s.MergePullRequests(ctx)
	lines = strings.Split(output.String(), "\n")
	assert.Equal("MERGED   1 : test commit 1", lines[0])
	assert.Equal("MERGED   1 : test commit 2", lines[1])
	fmt.Printf("OUT: %s\n", output.String())
	output.Reset()
}

func TestSPRReorderCommit(t *testing.T) {
	s, gitmock, githubmock, _, output := makeTestObjects(t)
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

	// 'git spr -s' :: StatusPullRequest
	githubmock.ExpectGetInfo()
	s.StatusPullRequests(ctx)
	assert.Equal("pull request stack is empty\n", output.String())
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
	githubmock.ExpectCreatePullRequest(c1, nil)
	githubmock.ExpectCreatePullRequest(c2, &c1)
	githubmock.ExpectCreatePullRequest(c3, &c2)
	githubmock.ExpectCreatePullRequest(c4, &c3)
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	fmt.Printf("OUT: %s\n", output.String())
	lines := strings.Split(output.String(), "\n")
	assert.Equal("[✔✔✔✔]   1 : test commit 4", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 3", lines[1])
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[2])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[3])
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c2, c4, c1, c3]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
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
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	fmt.Printf("OUT: %s\n", output.String())
	// TODO : Need to update pull requests in GetInfo expect to get this check to work
	// lines = strings.Split(output.String(), "\n")
	//assert.Equal("[✔✔✔✔]   1 : test commit 3", lines[0])
	//assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[1])
	//assert.Equal("[✔✔✔✔]   1 : test commit 4", lines[2])
	//assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[3])
	output.Reset()

	// TODO : add a call to merge and check merge order
}

func TestSPRDeleteCommit(t *testing.T) {
	s, gitmock, githubmock, _, output := makeTestObjects(t)
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

	// 'git spr -s' :: StatusPullRequest
	githubmock.ExpectGetInfo()
	s.StatusPullRequests(ctx)
	assert.Equal("pull request stack is empty\n", output.String())
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c1, c2, c3, c4]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
	gitmock.ExpectLogAndRespond([]*git.Commit{&c4, &c3, &c2, &c1})
	gitmock.ExpectPushCommits([]*git.Commit{&c1, &c2, &c3, &c4})
	githubmock.ExpectCreatePullRequest(c1, nil)
	githubmock.ExpectCreatePullRequest(c2, &c1)
	githubmock.ExpectCreatePullRequest(c3, &c2)
	githubmock.ExpectCreatePullRequest(c4, &c3)
	githubmock.ExpectGetInfo()

	s.UpdatePullRequests(ctx)
	fmt.Printf("OUT: %s\n", output.String())
	lines := strings.Split(output.String(), "\n")
	assert.Equal("[✔✔✔✔]   1 : test commit 4", lines[0])
	assert.Equal("[✔✔✔✔]   1 : test commit 3", lines[1])
	assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[2])
	assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[3])
	output.Reset()

	// 'git spr -u' :: UpdatePullRequest :: commits=[c2, c4, c1, c3]
	githubmock.ExpectGetInfo()
	gitmock.ExpectFetch()
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
	githubmock.ExpectGetInfo()
	s.UpdatePullRequests(ctx)
	fmt.Printf("OUT: %s\n", output.String())
	// TODO : Need to update pull requests in GetInfo expect to get this check to work
	// lines = strings.Split(output.String(), "\n")
	//assert.Equal("[✔✔✔✔]   1 : test commit 3", lines[0])
	//assert.Equal("[✔✔✔✔]   1 : test commit 1", lines[1])
	//assert.Equal("[✔✔✔✔]   1 : test commit 4", lines[2])
	//assert.Equal("[✔✔✔✔]   1 : test commit 2", lines[3])
	output.Reset()

	// TODO : add a call to merge and check merge order
}

func TestAmendNoCommits(t *testing.T) {
	s, gitmock, _, _, output := makeTestObjects(t)
	assert := require.New(t)
	ctx := context.Background()

	gitmock.ExpectLogAndRespond([]*git.Commit{})
	s.AmendCommit(ctx)
	assert.Equal("No commits to amend\n", output.String())
}

func TestAmendOneCommit(t *testing.T) {
	s, gitmock, _, input, output := makeTestObjects(t)
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
	assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend [1]: ", output.String())
}

func TestAmendTwoCommits(t *testing.T) {
	s, gitmock, _, input, output := makeTestObjects(t)
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
	assert.Equal(" 2 : 00000001 : test commit 1\n 1 : 00000002 : test commit 2\nCommit to amend [1-2]: ", output.String())
}

func TestAmendInvalidInput(t *testing.T) {
	s, gitmock, _, input, output := makeTestObjects(t)
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
	assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend [1]: Invalid input\n", output.String())
	output.Reset()

	gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
	input.WriteString("0")
	s.AmendCommit(ctx)
	assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend [1]: Invalid input\n", output.String())
	output.Reset()

	gitmock.ExpectLogAndRespond([]*git.Commit{&c1})
	input.WriteString("2")
	s.AmendCommit(ctx)
	assert.Equal(" 1 : 00000001 : test commit 1\nCommit to amend [1]: Invalid input\n", output.String())
	output.Reset()
}

func TestParseLocalCommitStack(t *testing.T) {
	var buffer bytes.Buffer
	sd := NewStackedPR(config.EmptyConfig(), nil, nil)
	sd.output = &buffer
	tests := []struct {
		name            string
		inputCommitLog  string
		expectedCommits []git.Commit
		expectedValid   bool
	}{
		{
			name: "SingleValidCommitNoBody",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed

	commit-id:053f6d16
`,
			expectedCommits: []git.Commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "",
				},
			},
			expectedValid: true,
		},
		{
			name: "SingleValidCommitWithBody",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed

	Super universe body.

	commit-id:053f6d16
`,
			expectedCommits: []git.Commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "Super universe body.",
				},
			},
			expectedValid: true,
		},
		{
			name: "TwoValidCommitsNoBody",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed

	commit-id:053f6d16

commit d604099d6604949e786e3d781919d43e46e88521 (origin/pr/ejoffe/master/39c84ea3)
Author: Hans Solo
Date:   Wed May 21 19:52:51 1980 -0700

	More engine power

	commit-id:39c84ea3
`,
			expectedCommits: []git.Commit{
				{
					CommitHash: "d604099d6604949e786e3d781919d43e46e88521",
					CommitID:   "39c84ea3",
					Subject:    "More engine power",
				},
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
				},
			},
			expectedValid: true,
		},
		{
			name: "SingleCommitMissingCommitID",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed
`,
			expectedCommits: nil,
			expectedValid:   false,
		},
	}

	for _, tc := range tests {
		actualCommits, valid := sd.parseLocalCommitStack(tc.inputCommitLog)
		assert.Equal(t, tc.expectedCommits, actualCommits, tc.name)
		assert.Equal(t, tc.expectedValid, valid, tc.name)
		if tc.expectedValid {
			assert.Equal(t, buffer.Len(), 0, tc.name)
		}
	}
}
