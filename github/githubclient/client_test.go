package githubclient

import (
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
			actual := matchPullRequestStack(repoConfig, "spr", "master", tc.commits, tc.prs)
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestComputeRequiredCheckStatus(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		contexts []checkContextNode
		expect   github.CheckStatus
	}{
		{
			name:     "NoChecks",
			contexts: []checkContextNode{},
			expect:   github.CheckStatusPass,
		},
		{
			name: "NoRequiredChecks_AllPass",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: false},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "NonRequiredFails_NoRequired",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Status: "COMPLETED", Conclusion: strPtr("FAILURE"), IsRequired: false},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "NonRequiredFails_RequiredPasses",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "required-ci", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: true},
				{TypeName: "CheckRun", Name: "optional-lint", Status: "COMPLETED", Conclusion: strPtr("FAILURE"), IsRequired: false},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "RequiredCheckFails",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "required-ci", Status: "COMPLETED", Conclusion: strPtr("FAILURE"), IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
		{
			name: "RequiredCheckPending",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "required-ci", Status: "IN_PROGRESS", IsRequired: true},
			},
			expect: github.CheckStatusPending,
		},
		{
			name: "RequiredCheckQueued",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "required-ci", Status: "QUEUED", IsRequired: true},
			},
			expect: github.CheckStatusPending,
		},
		{
			name: "RequiredNeutralPasses",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "info-check", Status: "COMPLETED", Conclusion: strPtr("NEUTRAL"), IsRequired: true},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "RequiredSkippedPasses",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "conditional", Status: "COMPLETED", Conclusion: strPtr("SKIPPED"), IsRequired: true},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "RequiredTimedOut",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "slow-test", Status: "COMPLETED", Conclusion: strPtr("TIMED_OUT"), IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
		{
			name: "RequiredCancelled",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "ci", Status: "COMPLETED", Conclusion: strPtr("CANCELLED"), IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
		{
			name: "StatusContext_RequiredSuccess",
			contexts: []checkContextNode{
				{TypeName: "StatusContext", Context: "ci/build", State: "SUCCESS", IsRequired: true},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "StatusContext_RequiredFailure",
			contexts: []checkContextNode{
				{TypeName: "StatusContext", Context: "ci/build", State: "FAILURE", IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
		{
			name: "StatusContext_RequiredPending",
			contexts: []checkContextNode{
				{TypeName: "StatusContext", Context: "ci/build", State: "PENDING", IsRequired: true},
			},
			expect: github.CheckStatusPending,
		},
		{
			name: "StatusContext_RequiredExpected",
			contexts: []checkContextNode{
				{TypeName: "StatusContext", Context: "ci/build", State: "EXPECTED", IsRequired: true},
			},
			expect: github.CheckStatusPending,
		},
		{
			name: "StatusContext_NonRequiredFails_RequiredPasses",
			contexts: []checkContextNode{
				{TypeName: "StatusContext", Context: "ci/build", State: "SUCCESS", IsRequired: true},
				{TypeName: "StatusContext", Context: "optional/lint", State: "FAILURE", IsRequired: false},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "MixedTypes_NonRequiredFails_RequiredPasses",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "required-ci", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: true},
				{TypeName: "StatusContext", Context: "ci/build", State: "SUCCESS", IsRequired: true},
				{TypeName: "CheckRun", Name: "optional-lint", Status: "COMPLETED", Conclusion: strPtr("FAILURE"), IsRequired: false},
				{TypeName: "StatusContext", Context: "optional/coverage", State: "FAILURE", IsRequired: false},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "MixedTypes_RequiredFails",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "required-ci", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: true},
				{TypeName: "StatusContext", Context: "ci/build", State: "FAILURE", IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
		{
			name: "FailTakesPrecedenceOverPending",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "ci-1", Status: "COMPLETED", Conclusion: strPtr("FAILURE"), IsRequired: true},
				{TypeName: "CheckRun", Name: "ci-2", Status: "IN_PROGRESS", IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
		{
			name: "AllRequiredPass_MultipleChecks",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "ci-1", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: true},
				{TypeName: "CheckRun", Name: "ci-2", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: true},
				{TypeName: "CheckRun", Name: "ci-3", Status: "COMPLETED", Conclusion: strPtr("SUCCESS"), IsRequired: true},
			},
			expect: github.CheckStatusPass,
		},
		{
			name: "CompletedWithNilConclusion",
			contexts: []checkContextNode{
				{TypeName: "CheckRun", Name: "weird", Status: "COMPLETED", Conclusion: nil, IsRequired: true},
			},
			expect: github.CheckStatusFail,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := computeRequiredCheckStatus(tc.contexts)
			require.Equal(t, tc.expect, actual)
		})
	}
}
