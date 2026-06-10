// Layer 2 tests for the divergence detection I/O: ReadRemoteHeads driven
// against mockgit. In package git_test to avoid an import cycle with
// git/mockgit.
package git_test

import (
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/stretchr/testify/require"
)

func TestReadRemoteHeads_EmptyBranchList(t *testing.T) {
	mock := mockgit.NewMockGit(t)
	heads, err := git.ReadRemoteHeads(mock, "origin", nil, git.MaxDivergenceWalkDepth)
	require.NoError(t, err)
	require.Empty(t, heads)
	mock.ExpectationsMet()
}

func TestReadRemoteHeads_FetchesExplicitRefspecs(t *testing.T) {
	// Verify the batched fetch with explicit refspecs (the protection
	// against restricted remote.origin.fetch configurations).
	mock := mockgit.NewMockGit(t)
	branches := []string{"spr/master/11111111", "spr/master/22222222"}
	mock.ExpectFetchHeadRefs("origin", branches)
	mock.ExpectRemoteHeadLog("origin", "spr/master/11111111", 20, []mockgit.RemoteCommitFixture{
		{SHA: "sha-A", Author: "Alice", Subject: "first", Body: "commit-id: 11111111\n"},
	})
	mock.ExpectRemoteHeadLog("origin", "spr/master/22222222", 20, []mockgit.RemoteCommitFixture{
		{SHA: "sha-B", Author: "Alice", Subject: "second", Body: "commit-id: 22222222\n"},
	})

	heads, err := git.ReadRemoteHeads(mock, "origin", branches, 20)
	require.NoError(t, err)
	require.Len(t, heads, 2)
	require.Equal(t, "sha-A", heads["spr/master/11111111"].TipSHA)
	require.Equal(t, "sha-B", heads["spr/master/22222222"].TipSHA)
	mock.ExpectationsMet()
}

func TestReadRemoteHeads_ParsesWalkbackHistory(t *testing.T) {
	// 3-commit history, ordered tip-first. Verify each entry is parsed
	// with SHA / author / subject / body / extracted CommitID.
	mock := mockgit.NewMockGit(t)
	branches := []string{"spr/master/aaaaaaaa"}
	mock.ExpectFetchHeadRefs("origin", branches)
	mock.ExpectRemoteHeadLog("origin", "spr/master/aaaaaaaa", 20, []mockgit.RemoteCommitFixture{
		{SHA: "sha-tip", Author: "Bob", Subject: "fix typo", Body: ""},
		{SHA: "sha-mid", Author: "Bob", Subject: "lint",
			Body: "more body\nlines\n"},
		{SHA: "sha-base", Author: "Alice", Subject: "feat: thing",
			Body: "context here\n\ncommit-id: aaaaaaaa\n"},
	})

	heads, err := git.ReadRemoteHeads(mock, "origin", branches, 20)
	require.NoError(t, err)
	require.Len(t, heads, 1)
	h := heads["spr/master/aaaaaaaa"]
	require.Equal(t, "sha-tip", h.TipSHA)
	require.Len(t, h.History, 3)

	require.Equal(t, "sha-tip", h.History[0].SHA)
	require.Equal(t, "Bob", h.History[0].Author)
	require.Equal(t, "fix typo", h.History[0].Subject)
	require.Empty(t, h.History[0].CommitID, "tip has no commit-id trailer")

	require.Equal(t, "sha-mid", h.History[1].SHA)
	require.Equal(t, "more body\nlines", h.History[1].Body,
		"multi-line body preserved (NUL is the separator, not newline)")
	require.Empty(t, h.History[1].CommitID)

	require.Equal(t, "sha-base", h.History[2].SHA)
	require.Equal(t, "aaaaaaaa", h.History[2].CommitID,
		"commit-id trailer extracted from base commit")
	mock.ExpectationsMet()
}

func TestReadRemoteHeads_MissingBranchSkipped(t *testing.T) {
	// Branch absent on remote (e.g. PR was never pushed): log call fails,
	// detection silently omits it.
	mock := mockgit.NewMockGit(t)
	branches := []string{"spr/master/exists01", "spr/master/missing1"}
	mock.ExpectFetchHeadRefs("origin", branches)
	mock.ExpectRemoteHeadLog("origin", "spr/master/exists01", 20, []mockgit.RemoteCommitFixture{
		{SHA: "sha-E", Author: "Alice", Subject: "thing", Body: "commit-id: exists01\n"},
	})
	mock.ExpectRemoteHeadLogMissing("origin", "spr/master/missing1", 20)

	heads, err := git.ReadRemoteHeads(mock, "origin", branches, 20)
	require.NoError(t, err)
	require.Len(t, heads, 1, "absent branch must be omitted, not error")
	_, present := heads["spr/master/exists01"]
	require.True(t, present)
	mock.ExpectationsMet()
}

func TestReadRemoteHeads_DefaultDepthWhenZero(t *testing.T) {
	// Passing depth=0 falls back to MaxDivergenceWalkDepth in the log -n flag.
	mock := mockgit.NewMockGit(t)
	branches := []string{"spr/master/11111111"}
	mock.ExpectFetchHeadRefs("origin", branches)
	mock.ExpectRemoteHeadLog("origin", "spr/master/11111111", git.MaxDivergenceWalkDepth,
		[]mockgit.RemoteCommitFixture{
			{SHA: "sha-A", Author: "Alice", Subject: "x", Body: "commit-id: 11111111\n"},
		})

	heads, err := git.ReadRemoteHeads(mock, "origin", branches, 0)
	require.NoError(t, err)
	require.Len(t, heads, 1)
	mock.ExpectationsMet()
}

func TestReadRemoteHeads_RemoteNameThreadedThrough(t *testing.T) {
	// Non-default remote name (e.g. "upstream", or the user's fork
	// alias) is used in both the fetch refspecs and the log ref path.
	mock := mockgit.NewMockGit(t)
	branches := []string{"spr/master/11111111"}
	mock.ExpectFetchHeadRefs("myfork", branches)
	mock.ExpectRemoteHeadLog("myfork", "spr/master/11111111", 20, []mockgit.RemoteCommitFixture{
		{SHA: "sha-A", Author: "Alice", Subject: "x", Body: "commit-id: 11111111\n"},
	})

	heads, err := git.ReadRemoteHeads(mock, "myfork", branches, 20)
	require.NoError(t, err)
	require.Len(t, heads, 1)
	mock.ExpectationsMet()
}
