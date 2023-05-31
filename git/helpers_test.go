package git

import "testing"

func TestBranchNameRegex(t *testing.T) {
	tests := []struct {
		input  string
		commit string
	}{
		{input: "spr/deadbeef", commit: "deadbeef"},
	}

	for _, tc := range tests {
		matches := _branchNameRegex.FindStringSubmatch(tc.input)
		if tc.commit != matches[1] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.commit, matches[1])
		}
	}
}

func TestBranchNameWithTargetRegex(t *testing.T) {
	tests := []struct {
		input  string
		branch string
		commit string
	}{
		{input: "spr/b1/deadbeef", branch: "b1", commit: "deadbeef"},
	}

	for _, tc := range tests {
		matches := _branchNameWithTargetRegex.FindStringSubmatch(tc.input)
		if tc.branch != matches[1] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.branch, matches[1])
		}
		if tc.commit != matches[2] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.commit, matches[2])
		}
	}
}
