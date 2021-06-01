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
	}
	for i, testCase := range testCases {
		t.Logf("Testing %v %v", i, testCase.remote)
		repoOwner, repoName, match := getRepoDetailsFromRemote(testCase.remote)
		if repoOwner != testCase.repoOwner {
			t.Fatalf("Wrong \"repoOwner\" returned for test case %v", i)
		}
		if repoName != testCase.repoName {
			t.Fatalf("Wrong \"repoName\" returned for test case %v", i)
		}
		if match != testCase.match {
			t.Fatalf("Wrong \"match\" returned for test case %v", i)
		}
	}
}
