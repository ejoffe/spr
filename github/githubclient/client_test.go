package githubclient

import "testing"

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
