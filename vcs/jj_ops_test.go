package vcs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/ejoffe/spr/vcs/mockjj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeJjTestConfig() *config.Config {
	cfg := config.EmptyConfig()
	cfg.Repo.GitHubRemote = "origin"
	cfg.Repo.GitHubBranch = "main"
	cfg.Repo.MergeMethod = "squash"
	return cfg
}

// --- FetchAndRebase ---

func TestJjOpsFetchAndRebase(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	jjmock.ExpectFetch()
	jjmock.ExpectRebase("origin", "main")

	err := ops.FetchAndRebase(cfg)
	require.NoError(t, err)
	jjmock.ExpectationsMet()
}

func TestJjOpsFetchAndRebase_NoRebase(t *testing.T) {
	cfg := makeJjTestConfig()
	cfg.User.NoRebase = true
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	// Only fetch, no rebase
	jjmock.ExpectFetch()

	err := ops.FetchAndRebase(cfg)
	require.NoError(t, err)
	jjmock.ExpectationsMet()
}

// --- GetLocalCommitStack ---

func TestJjOpsGetLocalCommitStack_AllHaveIDs(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	c1 := &git.Commit{
		CommitID:   "00000001",
		CommitHash: "c100000000000000000000000000000000000000",
		ChangeID:   "jjchange1",
		Subject:    "test commit 1",
	}
	c2 := &git.Commit{
		CommitID:   "00000002",
		CommitHash: "c200000000000000000000000000000000000000",
		ChangeID:   "jjchange2",
		Subject:    "test commit 2",
	}

	jjmock.ExpectLogAndRespond([]*git.Commit{c1, c2})

	commits := ops.GetLocalCommitStack(cfg, nil)
	require.Len(t, commits, 2)
	assert.Equal(t, "00000001", commits[0].CommitID)
	assert.Equal(t, "jjchange1", commits[0].ChangeID)
	assert.Equal(t, "00000002", commits[1].CommitID)
	assert.Equal(t, "jjchange2", commits[1].ChangeID)
	jjmock.ExpectationsMet()
}

func TestJjOpsGetLocalCommitStack_WIPCommit(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	c1 := &git.Commit{
		CommitID:   "00000001",
		CommitHash: "c100000000000000000000000000000000000000",
		ChangeID:   "jjchange1",
		Subject:    "WIP not ready yet",
	}

	jjmock.ExpectLogAndRespond([]*git.Commit{c1})

	commits := ops.GetLocalCommitStack(cfg, nil)
	require.Len(t, commits, 1)
	assert.True(t, commits[0].WIP)
	jjmock.ExpectationsMet()
}

// --- AmendInto ---

func TestJjOpsAmendInto(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	commit := git.Commit{
		CommitID:   "00000001",
		CommitHash: "c100000000000000000000000000000000000000",
		ChangeID:   "jjchange1",
	}

	jjmock.ExpectSquash("jjchange1")

	err := ops.AmendInto(commit)
	require.NoError(t, err)
	jjmock.ExpectationsMet()
}

func TestJjOpsAmendInto_NoChangeID(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	commit := git.Commit{
		CommitID:   "00000001",
		CommitHash: "c100000000000000000000000000000000000000",
		// No ChangeID
	}

	err := ops.AmendInto(commit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no jj change ID")
	jjmock.ExpectationsMet()
}

// --- EditStart / EditFinish / EditAbort ---

func TestJjOpsEditStart(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	gitmock := mockgit.NewMockGit(t)
	ops := NewJjOps(cfg, jjmock, gitmock)

	// Create a temp dir so EditStatePath works
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)
	// Override gitcmd.RootDir to use tmpDir
	ops.gitcmd = &mockRootDir{rootDir: tmpDir}

	commit := git.Commit{
		CommitID:   "00000001",
		CommitHash: "c100000000000000000000000000000000000000",
		ChangeID:   "jjchange1",
		Subject:    "test commit 1",
	}

	jjmock.ExpectOpLog("op123456abcdef")
	jjmock.ExpectLogAt("currentchange")
	jjmock.ExpectEdit("jjchange1")

	err := ops.EditStart(commit)
	require.NoError(t, err)
	assert.True(t, ops.IsEditing())

	// Verify state file contents
	data, err := os.ReadFile(ops.EditStatePath())
	require.NoError(t, err)
	assert.Contains(t, string(data), "original_at=currentchange")
	assert.Contains(t, string(data), "op_id=op123456abcdef")
	assert.Contains(t, string(data), "change_id=jjchange1")

	jjmock.ExpectationsMet()
}

func TestJjOpsEditFinish(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	// Create state file manually
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)
	ops.gitcmd = &mockRootDir{rootDir: tmpDir}

	stateContent := "vcs=jj\nchange_id=jjchange1\noriginal_at=prevchange\nop_id=op123456\ncommit_id=00000001\n"
	os.WriteFile(ops.EditStatePath(), []byte(stateContent), 0644)

	jjmock.ExpectNew("prevchange")

	err := ops.EditFinish()
	require.NoError(t, err)
	assert.False(t, ops.IsEditing()) // state file cleaned up
	jjmock.ExpectationsMet()
}

func TestJjOpsEditAbort(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)
	ops.gitcmd = &mockRootDir{rootDir: tmpDir}

	stateContent := "vcs=jj\nchange_id=jjchange1\noriginal_at=prevchange\nop_id=op123456\ncommit_id=00000001\n"
	os.WriteFile(ops.EditStatePath(), []byte(stateContent), 0644)

	jjmock.ExpectOpRestore("op123456")

	err := ops.EditAbort()
	require.NoError(t, err)
	assert.False(t, ops.IsEditing())
	jjmock.ExpectationsMet()
}

func TestJjOpsEditStart_NoChangeID(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	commit := git.Commit{CommitID: "00000001", CommitHash: "c100000000000000000000000000000000000000"}

	err := ops.EditStart(commit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no jj change ID")
	jjmock.ExpectationsMet()
}

// --- PrepareForPush ---

func TestJjOpsPrepareForPush_IsNoop(t *testing.T) {
	cfg := makeJjTestConfig()
	ops := NewJjOps(cfg, nil, nil)

	cleanup, err := ops.PrepareForPush()
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	cleanup() // should not panic
}

// --- CheckStackCompleteness ---

func TestJjOpsCheckStackCompleteness_AtTop(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	jjmock.ExpectCheckChildren("")

	warning := ops.CheckStackCompleteness()
	assert.Equal(t, "", warning)
	jjmock.ExpectationsMet()
}

func TestJjOpsCheckStackCompleteness_MidStack(t *testing.T) {
	cfg := makeJjTestConfig()
	jjmock := mockjj.NewMockJj(t)
	ops := NewJjOps(cfg, jjmock, nil)

	jjmock.ExpectCheckChildren("jjchange_above1\njjchange_above2")

	warning := ops.CheckStackCompleteness()
	assert.Contains(t, warning, "2 commit(s) above @")
	jjmock.ExpectationsMet()
}

// --- mockRootDir implements git.GitInterface just for RootDir ---

type mockRootDir struct {
	rootDir string
}

func (m *mockRootDir) GitWithEditor(args string, output *string, editorCmd string) error { return nil }
func (m *mockRootDir) Git(args string, output *string) error                             { return nil }
func (m *mockRootDir) MustGit(args string, output *string)                               {}
func (m *mockRootDir) RootDir() string                                                   { return m.rootDir }
func (m *mockRootDir) DeleteRemoteBranch(ctx context.Context, branch string) error       { return nil }
