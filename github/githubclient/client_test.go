package githubclient

import (
	"strings"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
	"github.com/stretchr/testify/require"
)

func TestMatchPullRequestStack(t *testing.T) {
	tests := []struct {
		name    string
		commits []git.Commit
		prs     genclient.PullRequestsRepositoryPullRequests
		expect  []*github.PullRequest
	}{
		{
			name:    "Empty",
			commits: []git.Commit{},
			prs:     genclient.PullRequestsRepositoryPullRequests{},
			expect:  []*github.PullRequest{},
		},
		{
			name:    "FirstCommit",
			commits: []git.Commit{{CommitID: "00000001"}},
			prs:     genclient.PullRequestsRepositoryPullRequests{},
			expect:  []*github.PullRequest{},
		},
		{
			name: "SecondCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
			},
			prs: genclient.PullRequestsRepositoryPullRequests{
				Nodes: &genclient.PullRequestsRepositoryPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/00000001",
						BaseRefName: "master",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "1",
					FromBranch: "spr/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
			},
		},
		{
			name: "ThirdCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
				{CommitID: "00000003"},
			},
			prs: genclient.PullRequestsRepositoryPullRequests{
				Nodes: &genclient.PullRequestsRepositoryPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/00000001",
						BaseRefName: "master",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/00000002",
						BaseRefName: "spr/00000001",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "2",
					FromBranch: "spr/00000002",
					ToBranch:   "spr/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
				{
					ID:         "1",
					FromBranch: "spr/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
			},
		},
		{
			name:    "RemoveOnlyCommit",
			commits: []git.Commit{},
			prs: genclient.PullRequestsRepositoryPullRequests{
				Nodes: &genclient.PullRequestsRepositoryPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/00000001",
						BaseRefName: "master",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{},
		},
		{
			name: "RemoveTopCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
			},
			prs: genclient.PullRequestsRepositoryPullRequests{
				Nodes: &genclient.PullRequestsRepositoryPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/00000001",
						BaseRefName: "master",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/00000002",
						BaseRefName: "spr/00000001",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/00000003",
						BaseRefName: "spr/00000002",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "2",
					FromBranch: "spr/00000002",
					ToBranch:   "spr/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
				{
					ID:         "1",
					FromBranch: "spr/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
			},
		},
		{
			name: "RemoveMiddleCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000003"},
			},
			prs: genclient.PullRequestsRepositoryPullRequests{
				Nodes: &genclient.PullRequestsRepositoryPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/00000001",
						BaseRefName: "master",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/00000002",
						BaseRefName: "spr/00000001",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/00000003",
						BaseRefName: "spr/00000002",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "3",
					FromBranch: "spr/00000003",
					ToBranch:   "spr/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
				{
					ID:         "2",
					FromBranch: "spr/00000002",
					ToBranch:   "spr/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
				{
					ID:         "1",
					FromBranch: "spr/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
			},
		},
		{
			name: "RemoveBottomCommit",
			commits: []git.Commit{
				{CommitID: "00000002"},
				{CommitID: "00000003"},
			},
			prs: genclient.PullRequestsRepositoryPullRequests{
				Nodes: &genclient.PullRequestsRepositoryPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/00000001",
						BaseRefName: "master",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/00000002",
						BaseRefName: "spr/00000001",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/00000003",
						BaseRefName: "spr/00000002",
						Commits: genclient.PullRequestsRepositoryPullRequestsNodesCommits{
							Nodes: &genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodes{
								{
									genclient.PullRequestsRepositoryPullRequestsNodesCommitsNodesCommit{Oid: "3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "3",
					FromBranch: "spr/00000003",
					ToBranch:   "spr/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
				{
					ID:         "2",
					FromBranch: "spr/00000002",
					ToBranch:   "spr/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
				{
					ID:         "1",
					FromBranch: "spr/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusFail,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := matchPullRequestStack("master", tc.commits, tc.prs)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestPullRequestRegex(t *testing.T) {
	tests := []struct {
		input  string
		commit string
	}{
		{input: "spr/deadbeef", commit: "deadbeef"},
	}

	for _, tc := range tests {
		matches := BranchNameRegex.FindStringSubmatch(tc.input)
		if tc.commit != matches[1] {
			t.Fatalf("expected: '%v', actual: '%v'", tc.commit, matches[1])
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

func TestInsertBodyIntoPRTemplateHappyPath(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		pullRequestTemplate string
		repo                *config.RepoConfig
		pr                  *github.PullRequest
		expected            string
	}{
		{
			name: "create PR",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr: nil,
			expected: `
## Related Issues
<!--- Add any related issues here -->

## Description
inserted body

## Checklist
- [ ] My code follows the style guidelines of this project`,
		},
		{
			name: "update PR",
			body: "updated description",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr: &github.PullRequest{
				Body: `
## Related Issues
* Issue #1234

## Description
original description

## Checklist
- [x] My code follows the style guidelines of this project`,
			},
			expected: `
## Related Issues
* Issue #1234

## Description
updated description

## Checklist
- [x] My code follows the style guidelines of this project`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := insertBodyIntoPRTemplate(tt.body, tt.pullRequestTemplate, tt.repo, tt.pr)
			if body != tt.expected {
				t.Fatalf("expected: '%v', actual: '%v'", tt.expected, body)
			}
		})
	}
}

func TestInsertBodyIntoPRTemplateErrors(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		pullRequestTemplate string
		repo                *config.RepoConfig
		pr                  *github.PullRequest
		expected            string
	}{
		{
			name: "no match insert start",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "does not exist",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr:       nil,
			expected: "no matches found: PR template insert start",
		},
		{
			name: "no match insert end",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "does not exist",
			},
			pr:       nil,
			expected: "no matches found: PR template insert end",
		},
		{
			name: "multiple many matches insert start",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here duplicate -->

## Description
<!--- Describe your changes in detail duplicate -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "duplicate",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr:       nil,
			expected: "multiple matches found: PR template insert start",
		},
		{
			name: "multiple many matches insert end",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here duplicate -->

## Description
<!--- Describe your changes in detail duplicate -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "duplicate",
			},
			pr:       nil,
			expected: "multiple matches found: PR template insert end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := insertBodyIntoPRTemplate(tt.body, tt.pullRequestTemplate, tt.repo, tt.pr)
			if !strings.Contains(err.Error(), tt.expected) {
				t.Fatalf("expected: '%v', actual: '%v'", tt.expected, err.Error())
			}
		})
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
