package config

import (
	"testing"

	"github.com/ejoffe/spr/git/mockgit"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/stretchr/testify/assert"
)

func TestGetRepoDetailsFromRemote(t *testing.T) {
	type testCase struct {
		remote     string
		githubHost string
		repoOwner  string
		repoName   string
		match      bool
	}
	testCases := []testCase{
		{"origin  https://github.com/r2/d2.git (push)", "github.com", "r2", "d2", true},
		{"origin  https://github.com/r2/d2.git (fetch)", "", "", "", false},
		{"origin  https://github.com/r2/d2 (push)", "github.com", "r2", "d2", true},

		{"origin  ssh://git@github.com/r2/d2.git (push)", "github.com", "r2", "d2", true},
		{"origin  ssh://git@github.com/r2/d2.git (fetch)", "", "", "", false},
		{"origin  ssh://git@github.com/r2/d2 (push)", "github.com", "r2", "d2", true},

		{"origin  git@github.com:r2/d2.git (push)", "github.com", "r2", "d2", true},
		{"origin  git@github.com:r2/d2.git (fetch)", "", "", "", false},
		{"origin  git@github.com:r2/d2 (push)", "github.com", "r2", "d2", true},

		{"origin  git@gh.enterprise.com:r2/d2.git (push)", "gh.enterprise.com", "r2", "d2", true},
		{"origin  git@gh.enterprise.com:r2/d2.git (fetch)", "", "", "", false},
		{"origin  git@gh.enterprise.com:r2/d2 (push)", "gh.enterprise.com", "r2", "d2", true},

		{"origin  https://github.com/r2/d2-a.git (push)", "github.com", "r2", "d2-a", true},
		{"origin  https://github.com/r2/d2_a.git (push)", "github.com", "r2", "d2_a", true},
	}
	for i, testCase := range testCases {
		t.Logf("Testing %v %q", i, testCase.remote)
		githubHost, repoOwner, repoName, match := getRepoDetailsFromRemote(testCase.remote)
		if githubHost != testCase.githubHost {
			t.Fatalf("Wrong \"githubHost\" returned for test case %v, expected %q, got %q", i, testCase.githubHost, githubHost)
		}
		if repoOwner != testCase.repoOwner {
			t.Fatalf("Wrong \"repoOwner\" returned for test case %v, expected %q, got %q", i, testCase.repoOwner, repoOwner)
		}
		if repoName != testCase.repoName {
			t.Fatalf("Wrong \"repoName\" returned for test case %v, expected %q, got %q", i, testCase.repoName, repoName)
		}
		if match != testCase.match {
			t.Fatalf("Wrong \"match\" returned for test case %v, expected %t, got %t", i, testCase.match, match)
		}
	}
}

func TestEmptyConfig(t *testing.T) {
	expect := &Config{
		Repo: &RepoConfig{},
		User: &UserConfig{},
		Internal: &InternalConfig{
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
			GitHubRemote:          "origin",
			GitHubBranch:          "master",
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
			Stargazer:        false,
			RunCount:         0,
		},
		Internal: &InternalConfig{
			MergeCheckCommit: map[string]string{},
		},
	}
	actual := DefaultConfig()
	assert.Equal(t, expect, actual)
}

func TestGitHubRemoteSource(t *testing.T) {
	mock := mockgit.NewMockGit(t)
	mock.ExpectRemote("https://github.com/r2/d2.git")

	expect := Config{
		Repo: &RepoConfig{
			GitHubRepoOwner: "r2",
			GitHubRepoName:  "d2",
			GitHubHost:      "github.com",
			RequireChecks:   false,
			RequireApproval: false,
			GitHubRemote:    "",
			GitHubBranch:    "",
			MergeMethod:     "",
		},
		User: &UserConfig{
			ShowPRLink:       false,
			LogGitCommands:   false,
			LogGitHubCalls:   false,
			StatusBitsHeader: false,
			Stargazer:        false,
			RunCount:         0,
		},
	}

	actual := Config{
		Repo: &RepoConfig{},
		User: &UserConfig{},
	}
	source := GitHubRemoteSource(&actual, mock)
	source.Load(nil)
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
