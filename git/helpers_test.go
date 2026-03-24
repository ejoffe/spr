package git

import (
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/stretchr/testify/assert"
)

func TestBranchNameRegex(t *testing.T) {
	tests := []struct {
		prefix string
		input  string
		branch string
		commit string
	}{
		{prefix: "spr", input: "spr/b1/deadbeef", branch: "b1", commit: "deadbeef"},
		{prefix: "spr", input: "spr/main/abcd1234", branch: "main", commit: "abcd1234"},
		{prefix: "custom", input: "custom/main/deadbeef", branch: "main", commit: "deadbeef"},
		{prefix: "my-team", input: "my-team/develop/abcd1234", branch: "develop", commit: "abcd1234"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			matches := BranchNameRegex(tc.prefix).FindStringSubmatch(tc.input)
			assert.NotNil(t, matches)
			assert.Equal(t, tc.branch, matches[1])
			assert.Equal(t, tc.commit, matches[2])
		})
	}
}

func TestBranchNameRegexNoMatch(t *testing.T) {
	tests := []struct {
		prefix string
		input  string
	}{
		{prefix: "spr", input: "other/main/deadbeef"},
		{prefix: "custom", input: "spr/main/deadbeef"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			matches := BranchNameRegex(tc.prefix).FindStringSubmatch(tc.input)
			assert.Nil(t, matches)
		})
	}
}

func TestBranchNameFromCommit(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		branch   string
		commitID string
		expected string
	}{
		{
			name:     "default prefix",
			prefix:   "spr",
			branch:   "main",
			commitID: "deadbeef",
			expected: "spr/main/deadbeef",
		},
		{
			name:     "custom prefix",
			prefix:   "my-team",
			branch:   "develop",
			commitID: "abcd1234",
			expected: "my-team/develop/abcd1234",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.EmptyConfig()
			cfg.User.BranchPrefix = tc.prefix
			cfg.Repo.GitHubBranch = tc.branch

			commit := Commit{CommitID: tc.commitID}
			result := BranchNameFromCommit(cfg, commit)
			assert.Equal(t, tc.expected, result)
		})
	}
}
