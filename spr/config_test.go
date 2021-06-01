package spr

import (
	"testing"
)

func TestGetRepoDetailsFromRemote(t *testing.T) {
	type testCase struct {
		remote    string
		repoOwner string
		repoName  string
		match     bool
	}
	testCases := []testCase{
		{"origin  https://github.com/r2/d2.git (push)", "r2", "d2", true},
		{"origin  https://github.com/r2/d2.git (fetch)", "", "", false},
		{"origin  https://github.com/r2/d2 (push)", "r2", "d2", true},

		{"origin  ssh://git@github.com/r2/d2.git (push)", "r2", "d2", true},
		{"origin  ssh://git@github.com/r2/d2.git (fetch)", "", "", false},
		{"origin  ssh://git@github.com/r2/d2 (push)", "r2", "d2", true},

		{"origin  git@github.com/r2/d2.git (push)", "r2", "d2", true},
		{"origin  git@github.com/r2/d2.git (fetch)", "", "", false},
		{"origin  git@github.com/r2/d2 (push)", "r2", "d2", true},
	}
	for i, testCase := range testCases {
		t.Logf("Testing %v %v", i, testCase.remote)
		repoOwner, repoName, match := getRepoDetailsFromRemote(testCase.remote)
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
