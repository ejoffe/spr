package githubclient

import (
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

func TestPullRequestRegex(t *testing.T) {
	tests := []struct {
		input  string
		branch string
		commit string
	}{
		{input: "pr/username/branchname/deadbeef", branch: "branchname", commit: "deadbeef"},
		{input: "pr/username/branch/name/deadbeef", branch: "branch/name", commit: "deadbeef"},
	}

	for _, tc := range tests {
		matches := BranchNameRegex.FindStringSubmatch(tc.input)
		if tc.branch != matches[1] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.branch, matches[1])
		}
		if tc.commit != matches[2] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.commit, matches[2])
		}
	}
}

func TestFormatPullRequestBody(t *testing.T) {
	simpleCommit := git.Commit{
		CommitID:   "abc123",
		CommitHash: "abcdef123456",
	}
	descriptiveCommit := git.Commit{
		CommitID:   "def456",
		CommitHash: "ghijkl7890",
		Body: `This body describes my nice PR.
It even includes some **markdown** formatting.`}

	tests := []struct {
		description string
		commit      git.Commit
		stack       []*github.PullRequest
	}{
		{
			description: "",
			commit:      git.Commit{},
			stack:       []*github.PullRequest{},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 2, Commit: descriptiveCommit},
			},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.

---

**Stack**:
- #2 ⬅
- #1


⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 1, Commit: simpleCommit},
				{Number: 2, Commit: descriptiveCommit},
			},
		},
	}

	for _, tc := range tests {
		body := formatBody(tc.commit, tc.stack)
		if body != tc.description {
			t.Fatalf("expected: '%v', actual: '%v'", tc.description, body)
		}
	}
}

func TestSortPullRequests(t *testing.T) {
	prs := []*github.PullRequest{
		{
			Number:     3,
			FromBranch: "third",
			ToBranch:   "second",
		},
		{
			Number:     2,
			FromBranch: "second",
			ToBranch:   "first",
		},
		{
			Number:     1,
			FromBranch: "first",
			ToBranch:   "master",
		},
	}

	config := config.DefaultConfig()
	prs = sortPullRequests(prs, config, "master")
	if prs[0].Number != 1 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[1].Number != 2 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[2].Number != 3 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
}

func TestSortPullRequestsMixed(t *testing.T) {
	prs := []*github.PullRequest{
		{
			Number:     3,
			FromBranch: "third",
			ToBranch:   "second",
		},
		{
			Number:     1,
			FromBranch: "first",
			ToBranch:   "master",
		},
		{
			Number:     2,
			FromBranch: "second",
			ToBranch:   "first",
		},
	}

	config := config.DefaultConfig()
	prs = sortPullRequests(prs, config, "master")
	if prs[0].Number != 1 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[1].Number != 2 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
	if prs[2].Number != 3 {
		t.Fatalf("prs not sorted correctly %v\n", prs)
	}
}
