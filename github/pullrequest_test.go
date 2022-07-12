package github

import (
	"fmt"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/stretchr/testify/assert"
)

func TestMergable(t *testing.T) {
	type testcase struct {
		pr     *PullRequest
		cfg    *config.Config
		expect bool
	}

	cfg := func(requireChecks bool, requireApproval bool) *config.Config {
		return &config.Config{
			Repo: &config.RepoConfig{
				RequireChecks:   requireChecks,
				RequireApproval: requireApproval,
			},
		}
	}

	pr := func(checks checkStatus, approved bool, noConflics bool, stacked bool) *PullRequest {
		return &PullRequest{
			MergeStatus: PullRequestMergeStatus{
				ChecksPass:     checks,
				ReviewApproved: approved,
				NoConflicts:    noConflics,
				Stacked:        stacked,
			},
		}
	}

	tests := []testcase{
		{pr(CheckStatusUnknown, false, false, false), cfg(false, false), false},
		{pr(CheckStatusUnknown, false, true, false), cfg(false, false), false},
		{pr(CheckStatusUnknown, false, true, false), cfg(false, false), false},
		{pr(CheckStatusUnknown, false, true, true), cfg(false, false), true},
		{pr(CheckStatusUnknown, false, true, true), cfg(true, false), false},
		{pr(CheckStatusPending, false, true, true), cfg(true, false), false},
		{pr(CheckStatusFail, false, true, true), cfg(true, false), false},
		{pr(CheckStatusPass, false, true, true), cfg(true, false), true},
		{pr(CheckStatusPass, false, true, true), cfg(true, true), false},
		{pr(CheckStatusPass, true, true, true), cfg(true, true), true},
	}
	for i, test := range tests {
		assert.Equal(t, test.expect, test.pr.Mergeable(test.cfg), fmt.Sprintf("case %d failed", i))
	}
}

func TestReady(t *testing.T) {
	type testcase struct {
		pr     *PullRequest
		cfg    *config.Config
		expect bool
	}

	cfg := func(requireChecks bool, requireApproval bool) *config.Config {
		return &config.Config{
			Repo: &config.RepoConfig{
				RequireChecks:   requireChecks,
				RequireApproval: requireApproval,
			},
		}
	}

	pr := func(checks checkStatus, wip bool, approved bool, noConflics bool, stacked bool) *PullRequest {
		return &PullRequest{
			MergeStatus: PullRequestMergeStatus{
				ChecksPass:     checks,
				ReviewApproved: approved,
				NoConflicts:    noConflics,
				Stacked:        stacked,
			},
			Commit: git.Commit{
				WIP: wip,
			},
		}
	}

	tests := []testcase{
		{pr(CheckStatusUnknown, false, false, true, false), cfg(false, false), true},
		{pr(CheckStatusPass, false, true, true, false), cfg(true, false), true},
		{pr(CheckStatusPass, false, true, false, false), cfg(true, true), false},
		{pr(CheckStatusFail, false, false, false, false), cfg(true, true), false},
		{pr(CheckStatusPass, true, false, false, false), cfg(true, true), false},
		{pr(CheckStatusPass, false, true, false, false), cfg(true, true), false},
		{pr(CheckStatusPass, false, false, true, false), cfg(true, true), false},
		{pr(CheckStatusPass, false, false, false, true), cfg(true, true), false},
	}
	for i, test := range tests {
		assert.Equal(t, test.expect, test.pr.Ready(test.cfg), fmt.Sprintf("case %d failed", i))
	}
}

func TestStatusString(t *testing.T) {
	type testcase struct {
		pr     *PullRequest
		cfg    *config.Config
		expect string
	}

	cfg := func(requireChecks bool, requireApproval bool) *config.Config {
		return &config.Config{
			Repo: &config.RepoConfig{
				RequireChecks:   requireChecks,
				RequireApproval: requireApproval,
			},
			User: &config.UserConfig{
				StatusBitsEmojis: false,
			},
		}
	}

	pr := func(checks checkStatus, approved bool, noConflics bool, stacked bool) *PullRequest {
		return &PullRequest{
			MergeStatus: PullRequestMergeStatus{
				ChecksPass:     checks,
				ReviewApproved: approved,
				NoConflicts:    noConflics,
				Stacked:        stacked,
			},
		}
	}

	tests := []testcase{
		{pr(CheckStatusPass, true, true, true), cfg(true, true), "[✔✔✔✔]"},
		{pr(CheckStatusFail, true, true, true), cfg(true, true), "[✗✔✔✔]"},
		{pr(CheckStatusUnknown, true, true, true), cfg(true, true), "[?✔✔✔]"},
		{pr(CheckStatusPending, true, true, true), cfg(true, true), "[·✔✔✔]"},
		{pr(CheckStatusPass, false, true, true), cfg(true, true), "[✔✗✔✔]"},
		{pr(CheckStatusPass, true, false, true), cfg(true, true), "[✔✔✗✔]"},
		{pr(CheckStatusPass, true, true, false), cfg(true, true), "[✔✔✔✗]"},
		{pr(CheckStatusPass, true, true, true), cfg(false, true), "[-✔✔✔]"},
		{pr(CheckStatusPass, true, true, true), cfg(false, false), "[--✔✔]"},
	}
	for i, test := range tests {
		assert.Equal(t, test.expect, test.pr.StatusString(test.cfg), fmt.Sprintf("case %d failed", i))
	}
}
