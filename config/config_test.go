package config

import (
	"testing"

	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/stretchr/testify/assert"
)

func TestEmptyConfig(t *testing.T) {
	expect := &Config{
		Repo:     &RepoConfig{},
		User:     &UserConfig{},
		Internal: &InternalConfig{},
		State: &InternalState{
			MergeCheckCommit: map[string]string{},
		},
	}
	actual := EmptyConfig()
	assert.Equal(t, expect, actual)
}

func TestDefaultConfig(t *testing.T) {
	expect := &Config{
		Repo: &RepoConfig{
			GitHubRepoOwner:       "",
			GitHubRepoName:        "",
			GitHubHost:            "github.com",
			RequireChecks:         true,
			RequireApproval:       true,
			MergeMethod:           "rebase",
			PRTemplatePath:        "",
			PRTemplateInsertStart: "",
			PRTemplateInsertEnd:   "",
		},
		User: &UserConfig{
			ShowPRLink:       true,
			LogGitCommands:   false,
			LogGitHubCalls:   false,
			StatusBitsHeader: true,
			StatusBitsEmojis: true,
		},
		Internal: &InternalConfig{
			GitHubRemote: "origin",
			GitHubBranch: "main",
		},
		State: &InternalState{
			MergeCheckCommit: map[string]string{},
		},
	}
	actual := DefaultConfig()
	assert.Equal(t, expect, actual)
}

func TestMergeMethodHelper(t *testing.T) {
	for _, tc := range []struct {
		configValue string
		expected    genclient.PullRequestMergeMethod
	}{
		{
			configValue: "rebase",
			expected:    genclient.PullRequestMergeMethod_REBASE,
		},
		{
			configValue: "",
			expected:    genclient.PullRequestMergeMethod_REBASE,
		},
		{
			configValue: "Merge",
			expected:    genclient.PullRequestMergeMethod_MERGE,
		},
		{
			configValue: "SQUASH",
			expected:    genclient.PullRequestMergeMethod_SQUASH,
		},
	} {
		tcName := tc.configValue
		if tcName == "" {
			tcName = "<EMPTY>"
		}
		t.Run(tcName, func(t *testing.T) {
			config := &Config{Repo: &RepoConfig{MergeMethod: tc.configValue}}
			actual, err := config.MergeMethod()
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
	t.Run("invalid", func(t *testing.T) {
		config := &Config{Repo: &RepoConfig{MergeMethod: "magic"}}
		actual, err := config.MergeMethod()
		assert.Error(t, err)
		assert.Empty(t, actual)
	})
}
