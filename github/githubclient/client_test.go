package githubclient

import (
	"context"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/githubclient/fezzik_types"
	"github.com/stretchr/testify/require"
)

func TestMatchPullRequestStack(t *testing.T) {
	tests := []struct {
		name    string
		commits []git.Commit
		prs     fezzik_types.PullRequestConnection
		expect  []*github.PullRequest
	}{
		{
			name: "ThirdCommitQueue",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
				{CommitID: "00000003"},
			},
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:              "2",
						HeadRefName:     "spr/master/00000002",
						BaseRefName:     "master",
						MergeQueueEntry: &fezzik_types.PullRequestsViewerPullRequestsNodesMergeQueueEntry{Id: "020"},
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1", MessageBody: "commit-id:1"},
								},
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2", MessageBody: "commit-id:2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "2",
					FromBranch: "spr/master/00000002",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
						Body:       "commit-id:2",
					},
					InQueue: true,
					Commits: []git.Commit{
						{CommitID: "1", CommitHash: "1", Body: "commit-id:1"},
						{CommitID: "2", CommitHash: "2", Body: "commit-id:2"},
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name: "FourthCommitQueue",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
				{CommitID: "00000003"},
				{CommitID: "00000004"},
			},
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:              "2",
						HeadRefName:     "spr/master/00000002",
						BaseRefName:     "master",
						MergeQueueEntry: &fezzik_types.PullRequestsViewerPullRequestsNodesMergeQueueEntry{Id: "020"},
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1", MessageBody: "commit-id:1"},
								},
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2", MessageBody: "commit-id:2"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "3", MessageBody: "commit-id:3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "2",
					FromBranch: "spr/master/00000002",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
						Body:       "commit-id:2",
					},
					InQueue: true,
					Commits: []git.Commit{
						{CommitID: "1", CommitHash: "1", Body: "commit-id:1"},
						{CommitID: "2", CommitHash: "2", Body: "commit-id:2"},
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					ID:         "3",
					FromBranch: "spr/master/00000003",
					ToBranch:   "spr/master/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
						Body:       "commit-id:3",
					},
					Commits: []git.Commit{
						{CommitID: "3", CommitHash: "3", Body: "commit-id:3"},
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name:    "Empty",
			commits: []git.Commit{},
			prs:     fezzik_types.PullRequestConnection{},
			expect:  []*github.PullRequest{},
		},
		{
			name:    "FirstCommit",
			commits: []git.Commit{{CommitID: "00000001"}},
			prs:     fezzik_types.PullRequestConnection{},
			expect:  []*github.PullRequest{},
		},
		{
			name: "SecondCommit",
			commits: []git.Commit{
				{CommitID: "00000001"},
				{CommitID: "00000002"},
			},
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "1",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
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
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "1",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					ID:         "2",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
		{
			name:    "RemoveOnlyCommit",
			commits: []git.Commit{},
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1"},
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
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "1",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					ID:         "2",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
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
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "1",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					ID:         "2",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					ID:         "3",
					FromBranch: "spr/master/00000003",
					ToBranch:   "spr/master/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
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
			prs: fezzik_types.PullRequestConnection{
				Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodes{
					{
						Id:          "1",
						HeadRefName: "spr/master/00000001",
						BaseRefName: "master",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "1"},
								},
							},
						},
					},
					{
						Id:          "2",
						HeadRefName: "spr/master/00000002",
						BaseRefName: "spr/master/00000001",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "2"},
								},
							},
						},
					},
					{
						Id:          "3",
						HeadRefName: "spr/master/00000003",
						BaseRefName: "spr/master/00000002",
						Commits: fezzik_types.PullRequestsViewerPullRequestsNodesCommits{
							Nodes: &fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodes{
								{
									fezzik_types.PullRequestsViewerPullRequestsNodesCommitsNodesCommit{Oid: "3"},
								},
							},
						},
					},
				},
			},
			expect: []*github.PullRequest{
				{
					ID:         "1",
					FromBranch: "spr/master/00000001",
					ToBranch:   "master",
					Commit: git.Commit{
						CommitID:   "00000001",
						CommitHash: "1",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},

				{
					ID:         "2",
					FromBranch: "spr/master/00000002",
					ToBranch:   "spr/master/00000001",
					Commit: git.Commit{
						CommitID:   "00000002",
						CommitHash: "2",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
				{
					ID:         "3",
					FromBranch: "spr/master/00000003",
					ToBranch:   "spr/master/00000002",
					Commit: git.Commit{
						CommitID:   "00000003",
						CommitHash: "3",
					},
					MergeStatus: github.PullRequestMergeStatus{
						ChecksPass: github.CheckStatusPass,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		repoConfig := &config.RepoConfig{}
		t.Run(tc.name, func(t *testing.T) {
			actual := matchPullRequestStack(repoConfig, "master", tc.commits, tc.prs)
			require.Equal(t, tc.expect, actual)
		})
	}
}

// TestGetInfoShouldAcceptLocalCommits verifies that GetInfo accepts a
// localCommits parameter so the caller can provide commits from VCSOperations
// (e.g. jj log) instead of GetInfo fetching them via git.GetLocalCommitStack.
//
// RED: currently GetInfo only takes (ctx, gitcmd) — no commits parameter.
// GREEN: after fix, GetInfo takes (ctx, gitcmd, localCommits).
func TestGetInfoShouldAcceptLocalCommits(t *testing.T) {
	// Define the interface we expect GetInfo to satisfy after the fix.
	type getInfoWithCommits interface {
		GetInfo(ctx context.Context, gitcmd git.GitInterface, localCommits []git.Commit) *github.GitHubInfo
	}

	cfg := config.EmptyConfig()
	c := &client{config: cfg}

	_, ok := interface{}(c).(getInfoWithCommits)
	require.True(t, ok,
		"GetInfo should accept a localCommits []git.Commit parameter so jj mode can provide commits")
}

func TestFormatTitle_StackNumber(t *testing.T) {
	prs := []*github.PullRequest{
		{Commit: git.Commit{CommitID: "aaa"}},
		{Commit: git.Commit{CommitID: "bbb"}},
		{Commit: git.Commit{CommitID: "ccc"}},
	}
	info := &github.GitHubInfo{PullRequests: prs}

	t.Run("disabled", func(t *testing.T) {
		cfg := config.EmptyConfig()
		c := &client{config: cfg}
		got := c.formatTitle("my title", git.Commit{CommitID: "bbb"}, info)
		require.Equal(t, "my title", got)
	})

	t.Run("enabled_middle", func(t *testing.T) {
		cfg := config.EmptyConfig()
		cfg.Repo.ShowStackNumberInTitle = true
		c := &client{config: cfg}
		got := c.formatTitle("my title", git.Commit{CommitID: "bbb"}, info)
		require.Equal(t, "[Stack 2/3] my title", got)
	})

	t.Run("enabled_first", func(t *testing.T) {
		cfg := config.EmptyConfig()
		cfg.Repo.ShowStackNumberInTitle = true
		c := &client{config: cfg}
		got := c.formatTitle("my title", git.Commit{CommitID: "aaa"}, info)
		require.Equal(t, "[Stack 1/3] my title", got)
	})

	t.Run("enabled_new_commit", func(t *testing.T) {
		cfg := config.EmptyConfig()
		cfg.Repo.ShowStackNumberInTitle = true
		c := &client{config: cfg}
		got := c.formatTitle("my title", git.Commit{CommitID: "zzz"}, info)
		require.Equal(t, "[Stack 4/4] my title", got)
	})
}
