package spr

import (
	"bytes"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/stretchr/testify/require"
)

// stubFoldStackedDiff wires a stackediff with a mock gitcmd and config
// preset to the merge policy. Defaults mirror what mockgit's helpers
// assume.
func stubFoldStackedDiff(t *testing.T) (*stackediff, *mockgit.Mock, *bytes.Buffer) {
	t.Helper()
	mock := mockgit.NewMockGit(t)
	cfg := config.EmptyConfig()
	cfg.Repo.OnRemoteDivergence = config.OnRemoteDivergenceMerge
	cfg.Repo.GitHubRemote = "origin"
	cfg.Repo.GitHubBranch = "master"
	out := &bytes.Buffer{}
	sd := &stackediff{
		config: cfg,
		gitcmd: mock,
		input:  &bytes.Buffer{},
		output: out,
	}
	return sd, mock, out
}

func foreignCommit(sha, subject string) git.RemoteCommit {
	return git.RemoteCommit{SHA: sha, Subject: subject, Author: "Maintainer"}
}

func TestFoldDivergences_NonForeignReason_Refused(t *testing.T) {
	sd, mock, _ := stubFoldStackedDiff(t)
	d := git.Divergence{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		Reason:      git.DivergenceCidMismatch,
		MismatchCID: "99999999",
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})
	require.Empty(t, folded)
	require.Len(t, refused, 1)
	require.Equal(t, git.DivergenceCidMismatch, refused[0].Reason)
	mock.ExpectationsMet()
}

func TestFoldDivergences_SinglePR_OneForeign_Succeeds(t *testing.T) {
	sd, mock, _ := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix typo")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})

	require.Len(t, folded, 1)
	require.Empty(t, refused)
	require.Equal(t, "spr/master/aaaaaaaa", folded[0].HeadBranch)
	mock.ExpectationsMet()
}

func TestFoldDivergences_SinglePR_MultipleForeign_AppliedOldestFirst(t *testing.T) {
	sd, mock, _ := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	// History tip is M2, M1 below; oldest (M1) goes first.
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCherryPickNoCommit("sha-M2")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{
			foreignCommit("sha-M2", "another fix"),
			foreignCommit("sha-M1", "fix typo"),
		},
		Reason: git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})

	require.Len(t, folded, 1)
	require.Empty(t, refused)
	mock.ExpectationsMet()
}

func TestFoldDivergences_MultiplePRs_TopDownOrder(t *testing.T) {
	// divs are returned bottom-first by DetectDivergence; fold processes
	// them in reverse so the upper PR (B) goes first. Lower-stack target
	// (A) keeps its original SHA when its turn comes.
	sd, mock, _ := stubFoldStackedDiff(t)

	// PR B (top) folds first.
	mock.ExpectRevParseHead("orig-head-1")
	mock.ExpectCheckoutDetach("sha-B")
	mock.ExpectCherryPickNoCommit("sha-MB1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-sha-B")
	mock.ExpectRebaseOnto("new-sha-B", "sha-B", "orig-head-1")

	// PR A (bottom) folds second; origHead is now the post-B tip.
	mock.ExpectRevParseHead("post-fold-B-head")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-MA1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-sha-A")
	mock.ExpectRebaseOnto("new-sha-A", "sha-A", "post-fold-B-head")

	divs := []git.Divergence{
		{
			HeadBranch:     "spr/master/aaaaaaaa",
			LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-MA1", "A's fix")},
			Reason:         git.DivergenceForeignCommits,
		},
		{
			HeadBranch:     "spr/master/bbbbbbbb",
			LocalCommit:    git.Commit{CommitID: "bbbbbbbb", CommitHash: "sha-B"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-MB1", "B's fix")},
			Reason:         git.DivergenceForeignCommits,
		},
	}
	folded, refused := sd.foldDivergences(divs)
	require.Len(t, folded, 2)
	require.Empty(t, refused)
	mock.ExpectationsMet()
}

func TestFoldDivergences_CherryPickConflict_RolledBackAndRefused(t *testing.T) {
	sd, mock, out := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommitConflict("sha-M1")
	mock.ExpectCherryPickAbort()
	mock.ExpectCheckout("orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})

	require.Empty(t, folded)
	require.Len(t, refused, 1)
	require.Contains(t, out.String(), "deferred",
		"conflict path should emit a per-PR warning")
	mock.ExpectationsMet()
}

func TestFoldDivergences_RebaseConflict_RolledBackAndRefused(t *testing.T) {
	sd, mock, out := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOntoConflict("new-target-sha", "sha-A", "orig-head-sha")
	mock.ExpectRebaseAbort()
	mock.ExpectCheckout("orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})

	require.Empty(t, folded)
	require.Len(t, refused, 1)
	require.Contains(t, out.String(), "deferred")
	mock.ExpectationsMet()
}

func TestFoldDivergences_MultiplePRs_OneConflicts_OtherProceeds(t *testing.T) {
	// Top-down order: PR B (top) tries first and conflicts; PR A still
	// folds cleanly after.
	sd, mock, _ := stubFoldStackedDiff(t)

	// PR B fails on cherry-pick.
	mock.ExpectRevParseHead("orig-head-1")
	mock.ExpectCheckoutDetach("sha-B")
	mock.ExpectCherryPickNoCommitConflict("sha-MB1")
	mock.ExpectCherryPickAbort()
	mock.ExpectCheckout("orig-head-1")

	// PR A succeeds; origHead is still orig-head-1 because B's fold
	// reverted.
	mock.ExpectRevParseHead("orig-head-1")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-MA1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-sha-A")
	mock.ExpectRebaseOnto("new-sha-A", "sha-A", "orig-head-1")

	divs := []git.Divergence{
		{
			HeadBranch:     "spr/master/aaaaaaaa",
			LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-MA1", "A fix")},
			Reason:         git.DivergenceForeignCommits,
		},
		{
			HeadBranch:     "spr/master/bbbbbbbb",
			LocalCommit:    git.Commit{CommitID: "bbbbbbbb", CommitHash: "sha-B"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-MB1", "B fix")},
			Reason:         git.DivergenceForeignCommits,
		},
	}
	folded, refused := sd.foldDivergences(divs)
	require.Len(t, folded, 1)
	require.Equal(t, "spr/master/aaaaaaaa", folded[0].HeadBranch)
	require.Len(t, refused, 1)
	require.Equal(t, "spr/master/bbbbbbbb", refused[0].HeadBranch)
	mock.ExpectationsMet()
}

func TestFoldDivergences_MixedReasons_OnlyForeignFolded(t *testing.T) {
	sd, mock, _ := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	divs := []git.Divergence{
		{
			HeadBranch:  "spr/master/bbbbbbbb",
			LocalCommit: git.Commit{CommitID: "bbbbbbbb", CommitHash: "sha-B"},
			Reason:      git.DivergenceCidNotFound,
		},
		{
			HeadBranch:     "spr/master/aaaaaaaa",
			LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix")},
			Reason:         git.DivergenceForeignCommits,
		},
	}
	folded, refused := sd.foldDivergences(divs)
	require.Len(t, folded, 1)
	require.Equal(t, "spr/master/aaaaaaaa", folded[0].HeadBranch)
	require.Len(t, refused, 1)
	require.Equal(t, "spr/master/bbbbbbbb", refused[0].HeadBranch)
	mock.ExpectationsMet()
}

func TestFoldDivergences_MissingTargetHash_Refused(t *testing.T) {
	sd, mock, _ := stubFoldStackedDiff(t)

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: ""},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})
	require.Empty(t, folded)
	require.Len(t, refused, 1)
	mock.ExpectationsMet()
}

func TestApplyDivergencePolicy_Merge_AllFolded_Proceeds(t *testing.T) {
	sd, mock, out := stubFoldStackedDiff(t)
	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix")},
		Reason:         git.DivergenceForeignCommits,
	}
	ok := sd.applyDivergencePolicy([]git.Divergence{d})
	require.True(t, ok, "all folded → proceed with update")
	require.Contains(t, out.String(), "Folded 1 PR")
	mock.ExpectationsMet()
}

func TestApplyDivergencePolicy_Merge_AnyRefused_RefusesUpdate(t *testing.T) {
	sd, mock, out := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendNoEdit()
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	divs := []git.Divergence{
		{
			HeadBranch:     "spr/master/aaaaaaaa",
			LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix")},
			Reason:         git.DivergenceForeignCommits,
		},
		{
			HeadBranch:  "spr/master/bbbbbbbb",
			LocalCommit: git.Commit{CommitID: "bbbbbbbb", CommitHash: "sha-B"},
			MismatchCID: "99999999",
			Reason:      git.DivergenceCidMismatch,
		},
	}
	ok := sd.applyDivergencePolicy(divs)
	require.False(t, ok, "anything refused → refuse update")
	require.Contains(t, out.String(), "Folded 1")
	require.Contains(t, out.String(), "Refused")
	require.Contains(t, out.String(), "Refusing to continue")
	mock.ExpectationsMet()
}
