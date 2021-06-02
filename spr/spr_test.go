package spr

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortPullRequests(t *testing.T) {
	prs := []*pullRequest{
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

	sd := NewStackedPR(&Config{}, nil, nil, false)
	prs = sd.sortPullRequests(prs)
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
	prs := []*pullRequest{
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

	sd := NewStackedPR(&Config{}, nil, nil, false)
	prs = sd.sortPullRequests(prs)
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
		matches := pullRequestRegex.FindStringSubmatch(tc.input)
		if tc.branch != matches[1] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.branch, matches[1])
		}
		if tc.commit != matches[2] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.commit, matches[2])
		}
	}
}

func TestParseLocalCommitStack(t *testing.T) {
	var buffer bytes.Buffer
	sd := NewStackedPR(&Config{}, nil, &buffer, false)
	tests := []struct {
		name                      string
		inputCommitLog            string
		expectedCommits           []commit
		expectedCommitHookMessage bool
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
			expectedCommits: []commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "",
				},
			},
			expectedCommitHookMessage: false,
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
			expectedCommits: []commit{
				{
					CommitHash: "d89e0e460ed817c81641f32b1a506b60164b4403",
					CommitID:   "053f6d16",
					Subject:    "Supergalactic speed",
					Body:       "Super universe body.",
				},
			},
			expectedCommitHookMessage: false,
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
			expectedCommits: []commit{
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
			expectedCommitHookMessage: false,
		},
		{
			name: "SingleCommitMissingCommitID",
			inputCommitLog: `
commit d89e0e460ed817c81641f32b1a506b60164b4403 (HEAD -> master)
Author: Han Solo
Date:   Wed May 21 19:53:12 1980 -0700

	Supergalactic speed
`,
			expectedCommits:           nil,
			expectedCommitHookMessage: true,
		},
	}

	for _, tc := range tests {
		actualCommits := sd.parseLocalCommitStack(tc.inputCommitLog)
		assert.Equal(t, tc.expectedCommits, actualCommits, tc.name)

		if tc.expectedCommitHookMessage {
			assert.Equal(t, buffer.String(), commitInstallHelper[1:])
		} else {
			assert.Equal(t, buffer.Len(), 0)
		}
	}
}
