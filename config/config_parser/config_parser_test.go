package config_parser

import (
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git/mockgit"
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
		{"origin  https://github.com/r-2/d-2 (push)", "github.com", "r-2", "d-2", true},

		{"origin  ssh://git@github.com/r2/d2.git (push)", "github.com", "r2", "d2", true},
		{"origin  ssh://git@github.com/r2/d2.git (fetch)", "", "", "", false},
		{"origin  ssh://git@github.com/r2/d2 (push)", "github.com", "r2", "d2", true},
		{"origin  ssh://git@github.com/r-2/d-2 (push)", "github.com", "r-2", "d-2", true},

		{"origin  git@github.com:r2/d2.git (push)", "github.com", "r2", "d2", true},
		{"origin  git@github.com:r2/d2.git (fetch)", "", "", "", false},
		{"origin  git@github.com:r2/d2 (push)", "github.com", "r2", "d2", true},
		{"origin  git@github.com:r-2/d-2 (push)", "github.com", "r-2", "d-2", true},

		{"origin  git@gh.enterprise.com:r2/d2.git (push)", "gh.enterprise.com", "r2", "d2", true},
		{"origin  git@gh.enterprise.com:r2/d2.git (fetch)", "", "", "", false},
		{"origin  git@gh.enterprise.com:r2/d2 (push)", "gh.enterprise.com", "r2", "d2", true},
		{"origin  git@gh.enterprise.com:r-2/d-2 (push)", "gh.enterprise.com", "r-2", "d-2", true},

		{"origin  https://github.com/r2/d2-a.git (push)", "github.com", "r2", "d2-a", true},
		{"origin  https://github.com/r-2/d2-a.git (push)", "github.com", "r-2", "d2-a", true},
		{"origin  https://github.com/r2/d2_a.git (push)", "github.com", "r2", "d2_a", true},
		{"origin  https://github.com/r-2/d2_a.git (push)", "github.com", "r-2", "d2_a", true},

		// GitHub names are case-sensitive
		{"origin  https://github.com/R2/D2.git (push)", "github.com", "R2", "D2", true},
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

func TestGitHubRemoteSource(t *testing.T) {
	mock := mockgit.NewMockGit(t)
	mock.ExpectRemote("https://github.com/r2/d2.git")

	expect := config.Config{
		Repo: &config.RepoConfig{
			GitHubRepoOwner: "r2",
			GitHubRepoName:  "d2",
			GitHubHost:      "github.com",
			RequireChecks:   false,
			RequireApproval: false,
			MergeMethod:     "",
		},
		User: &config.UserConfig{
			ShowPRLink:       false,
			LogGitCommands:   false,
			LogGitHubCalls:   false,
			StatusBitsHeader: false,
		},
	}

	actual := config.Config{
		Repo: &config.RepoConfig{},
		User: &config.UserConfig{},
	}
	source := NewGitHubRemoteSource(&actual, mock)
	source.Load(nil)
	assert.Equal(t, expect, actual)
	mock.ExpectationsMet()
}
