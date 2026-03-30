package vcs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJjLogOutput_SingleCommit(t *testing.T) {
	input := "c100000000000000000000000000000000000000\x1fmychangeid1234\x1ffalse\x1ftest commit 1\n\ncommit-id:00000001\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.True(t, valid)
	require.Len(t, commits, 1)
	assert.Equal(t, "00000001", commits[0].sprCommitID)
	assert.Equal(t, "mychangeid1234", commits[0].changeID)
	assert.Equal(t, "c100000000000000000000000000000000000000", commits[0].commitHash)
	assert.Equal(t, "test commit 1", commits[0].subject)
	assert.False(t, commits[0].wip)
	assert.False(t, commits[0].empty)
}

func TestParseJjLogOutput_MultipleCommits(t *testing.T) {
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ffalse\x1fcommit 1\n\ncommit-id:00000001\n\x1e" +
		"c200000000000000000000000000000000000000\x1fchange2\x1ffalse\x1fcommit 2\n\ncommit-id:00000002\n\x1e" +
		"c300000000000000000000000000000000000000\x1fchange3\x1ffalse\x1fcommit 3\n\ncommit-id:00000003\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.True(t, valid)
	require.Len(t, commits, 3)
	assert.Equal(t, "00000001", commits[0].sprCommitID)
	assert.Equal(t, "00000002", commits[1].sprCommitID)
	assert.Equal(t, "00000003", commits[2].sprCommitID)
	assert.Equal(t, "change1", commits[0].changeID)
	assert.Equal(t, "change2", commits[1].changeID)
	assert.Equal(t, "change3", commits[2].changeID)
}

func TestParseJjLogOutput_MissingCommitID(t *testing.T) {
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ffalse\x1fcommit without trailer\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.False(t, valid) // invalid because non-empty commit lacks commit-id
	require.Len(t, commits, 1)
	assert.Equal(t, "", commits[0].sprCommitID)
	assert.Equal(t, "change1", commits[0].changeID)
	assert.Equal(t, "commit without trailer", commits[0].subject)
}

func TestParseJjLogOutput_EmptyCommitSkipped(t *testing.T) {
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ftrue\x1f\x1e" +
		"c200000000000000000000000000000000000000\x1fchange2\x1ffalse\x1freal commit\n\ncommit-id:00000001\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.True(t, valid)
	require.Len(t, commits, 1) // empty commit skipped
	assert.Equal(t, "change2", commits[0].changeID)
}

func TestParseJjLogOutput_WIPPrefix(t *testing.T) {
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ffalse\x1fWIP work in progress\n\ncommit-id:00000001\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.True(t, valid)
	require.Len(t, commits, 1)
	assert.True(t, commits[0].wip)
	assert.Equal(t, "WIP work in progress", commits[0].subject)
}

func TestParseJjLogOutput_MultiLineBody(t *testing.T) {
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ffalse\x1fFix the bug\n\nThis is a detailed\ndescription of the fix.\n\ncommit-id:deadbeef\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.True(t, valid)
	require.Len(t, commits, 1)
	assert.Equal(t, "Fix the bug", commits[0].subject)
	assert.Equal(t, "deadbeef", commits[0].sprCommitID)
	assert.Contains(t, commits[0].body, "detailed")
	assert.Contains(t, commits[0].body, "description of the fix")
}

func TestParseJjLogOutput_EmptyInput(t *testing.T) {
	commits, valid := parseJjLogOutput("")
	require.True(t, valid) // no commits = valid (nothing to check)
	require.Len(t, commits, 0)
}

func TestParseJjLogOutput_CommitIDWithSpace(t *testing.T) {
	// commit-id: with a space after colon (spr regex allows this)
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ffalse\x1ftest commit\n\ncommit-id: abcdef01\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.True(t, valid)
	require.Len(t, commits, 1)
	assert.Equal(t, "abcdef01", commits[0].sprCommitID)
}

func TestParseJjLogOutput_MixedValidAndInvalid(t *testing.T) {
	// First commit has trailer, second doesn't
	input := "c100000000000000000000000000000000000000\x1fchange1\x1ffalse\x1fcommit 1\n\ncommit-id:00000001\n\x1e" +
		"c200000000000000000000000000000000000000\x1fchange2\x1ffalse\x1fcommit 2 no trailer\n\x1e"
	commits, valid := parseJjLogOutput(input)
	require.False(t, valid) // invalid because second commit lacks trailer
	require.Len(t, commits, 2)
	assert.Equal(t, "00000001", commits[0].sprCommitID)
	assert.Equal(t, "", commits[1].sprCommitID)
}
