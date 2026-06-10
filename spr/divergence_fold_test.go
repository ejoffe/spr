package spr

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/git/mockgit"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/mockclient"
	"github.com/stretchr/testify/require"
)

const testFoldMsgPath = "/tmp/spr-fold-msg-test.txt"

// stubFoldStackedDiff wires a stackediff with a mock gitcmd and config
// preset to the merge policy. Tests get a fixed foldMessageWriter that
// captures the message content for assertions.
func stubFoldStackedDiff(t *testing.T) (*stackediff, *mockgit.Mock, *bytes.Buffer, *string) {
	t.Helper()
	mock := mockgit.NewMockGit(t)
	cfg := config.EmptyConfig()
	cfg.Repo.OnRemoteDivergence = config.OnRemoteDivergenceMerge
	cfg.Repo.GitHubRemote = "origin"
	cfg.Repo.GitHubBranch = "master"
	out := &bytes.Buffer{}
	captured := new(string)
	sd := &stackediff{
		config: cfg,
		gitcmd: mock,
		input:  &bytes.Buffer{},
		output: out,
		foldMessageWriter: func(content string) (string, func(), error) {
			*captured = content
			return testFoldMsgPath, func() {}, nil
		},
	}
	return sd, mock, out, captured
}

func foreignCommit(sha, subject, author, email string) git.RemoteCommit {
	return git.RemoteCommit{SHA: sha, Subject: subject, Author: author, AuthorEmail: email}
}

func TestFoldDivergences_NonForeignReason_Refused(t *testing.T) {
	sd, mock, _, _ := stubFoldStackedDiff(t)
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
	sd, mock, _, captured := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\nbody\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix typo", "Maintainer", "m@example.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})

	require.Len(t, folded, 1)
	require.Empty(t, refused)
	require.Contains(t, *captured, "subject")
	require.Contains(t, *captured, "commit-id: aaaaaaaa")
	require.Contains(t, *captured, "Co-authored-by: Maintainer <m@example.com>")
	mock.ExpectationsMet()
}

func TestFoldDivergences_DedupsCoauthorTrailers(t *testing.T) {
	// Two foreign commits from the same author → one Co-authored-by line.
	sd, mock, _, captured := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCherryPickNoCommit("sha-M2")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{
			foreignCommit("sha-M2", "another fix", "M", "m@e.com"),
			foreignCommit("sha-M1", "fix typo", "M", "m@e.com"),
		},
		Reason: git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})
	require.Len(t, folded, 1)
	require.Empty(t, refused)
	// Exactly one Co-authored-by line for the duplicate author.
	require.Equal(t, 1, bytes.Count([]byte(*captured), []byte("Co-authored-by: M")))
	mock.ExpectationsMet()
}

func TestFoldDivergences_PreservesDistinctCoauthors(t *testing.T) {
	sd, mock, _, captured := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCherryPickNoCommit("sha-M2")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{
			foreignCommit("sha-M2", "another fix", "Bob", "bob@e.com"),
			foreignCommit("sha-M1", "fix typo", "Alice", "alice@e.com"),
		},
		Reason: git.DivergenceForeignCommits,
	}
	folded, _ := sd.foldDivergences([]git.Divergence{d})
	require.Len(t, folded, 1)
	require.Contains(t, *captured, "Co-authored-by: Alice <alice@e.com>")
	require.Contains(t, *captured, "Co-authored-by: Bob <bob@e.com>")
	mock.ExpectationsMet()
}

func TestFoldDivergences_CherryPickConflict_RolledBackAndRefused(t *testing.T) {
	sd, mock, out, _ := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommitConflict("sha-M1")
	mock.ExpectCherryPickAbort()
	mock.ExpectCheckout("orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})
	require.Empty(t, folded)
	require.Len(t, refused, 1)
	require.Contains(t, out.String(), "deferred")
	mock.ExpectationsMet()
}

func TestFoldDivergences_RebaseConflict_RolledBackAndRefused(t *testing.T) {
	sd, mock, out, _ := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOntoConflict("new-target-sha", "sha-A", "orig-head-sha")
	mock.ExpectRebaseAbort()
	mock.ExpectCheckout("orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})
	require.Empty(t, folded)
	require.Len(t, refused, 1)
	require.Contains(t, out.String(), "deferred")
	mock.ExpectationsMet()
}

func TestFoldDivergences_MissingTargetHash_Refused(t *testing.T) {
	sd, mock, _, _ := stubFoldStackedDiff(t)

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: ""},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	folded, refused := sd.foldDivergences([]git.Divergence{d})
	require.Empty(t, folded)
	require.Len(t, refused, 1)
	mock.ExpectationsMet()
}

func TestApplyDivergencePolicy_Merge_AllFolded_Proceeds(t *testing.T) {
	sd, mock, out, _ := stubFoldStackedDiff(t)
	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	ok := sd.applyDivergencePolicy(context.Background(), nil, []git.Divergence{d})
	require.True(t, ok, "all folded → proceed with update")
	require.Contains(t, out.String(), "Folded 1 PR")
	mock.ExpectationsMet()
}

func TestPostFoldComments_PostsOnePerFoldedPR(t *testing.T) {
	// Verify the merge policy posts a PR comment for each folded
	// divergence (and only for folded ones, not refused).
	sd, mock, _, _ := stubFoldStackedDiff(t)
	ghMock := mockclient.NewMockClient(t)
	sd.github = ghMock

	prAaaa := &github.PullRequest{ID: "id-aaaa", Number: 1,
		Commit: git.Commit{CommitID: "aaaaaaaa"}}
	info := &github.GitHubInfo{PullRequests: []*github.PullRequest{prAaaa}}

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")
	ghMock.ExpectCommentPullRequest(prAaaa.Commit)

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	ok := sd.applyDivergencePolicy(context.Background(), info, []git.Divergence{d})
	require.True(t, ok)
	mock.ExpectationsMet()
	ghMock.ExpectationsMet()
}

func TestPostFoldComments_NilInfo_NoCalls(t *testing.T) {
	// When info is nil (e.g. tests, or no GitHub state yet), the fold
	// still proceeds but no comments are posted.
	sd, mock, _, _ := stubFoldStackedDiff(t)
	ghMock := mockclient.NewMockClient(t)
	sd.github = ghMock

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	ok := sd.applyDivergencePolicy(context.Background(), nil, []git.Divergence{d})
	require.True(t, ok)
	mock.ExpectationsMet()
	// ghMock has no expectations queued and we made no calls — passes trivially.
}

func TestPostFoldComments_UnmatchedPR_Skipped(t *testing.T) {
	// A folded divergence whose commit-id doesn't match any PR in
	// GitHubInfo is skipped silently (no panic, no comment).
	sd, mock, _, _ := stubFoldStackedDiff(t)
	ghMock := mockclient.NewMockClient(t)
	sd.github = ghMock
	info := &github.GitHubInfo{PullRequests: []*github.PullRequest{
		{ID: "id-other", Number: 99, Commit: git.Commit{CommitID: "99999999"}},
	}}

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	d := git.Divergence{
		HeadBranch:     "spr/master/aaaaaaaa",
		LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
		ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
		Reason:         git.DivergenceForeignCommits,
	}
	ok := sd.applyDivergencePolicy(context.Background(), info, []git.Divergence{d})
	require.True(t, ok)
	mock.ExpectationsMet()
}

func TestBuildFoldComment_ContainsExpectedFields(t *testing.T) {
	d := git.Divergence{
		HeadBranch:  "spr/master/aaaaaaaa",
		LocalCommit: git.Commit{CommitID: "aaaaaaaa"},
		ForeignCommits: []git.RemoteCommit{
			{SHA: "abc1234567", Subject: "fix typo", Author: "Maintainer"},
		},
	}
	c := buildFoldComment(d)
	require.Contains(t, c, "spr integrated")
	require.Contains(t, c, "1 commit")
	require.Contains(t, c, "abc1234") // short SHA
	require.Contains(t, c, "fix typo")
	require.Contains(t, c, "Maintainer")
	require.Contains(t, c, "spr/master/aaaaaaaa")
	require.Contains(t, c, "Do **not** `git push --force`")
}

func TestAppendCoauthorTrailers_NoExistingTrailers(t *testing.T) {
	orig := "subject\n\nbody line\n"
	result := appendCoauthorTrailers(orig, []git.RemoteCommit{
		{Author: "M", AuthorEmail: "m@e.com"},
	})
	// Trailer block separated from body by a blank line.
	require.Contains(t, result, "body line\n\nCo-authored-by: M <m@e.com>")
}

func TestAppendCoauthorTrailers_ExistingCommitIDTrailer(t *testing.T) {
	// commit-id trailer already at end → new trailers go in the same block,
	// no extra blank line.
	orig := "subject\n\nbody\n\ncommit-id: aaaaaaaa\n"
	result := appendCoauthorTrailers(orig, []git.RemoteCommit{
		{Author: "M", AuthorEmail: "m@e.com"},
	})
	require.Contains(t, result, "commit-id: aaaaaaaa\nCo-authored-by: M <m@e.com>")
}

func TestAppendCoauthorTrailers_DedupAgainstExistingCoauthor(t *testing.T) {
	// If the message already has a Co-authored-by: trailer for the same
	// author, don't add it again (idempotent re-fold).
	orig := "subject\n\ncommit-id: aaaaaaaa\nCo-authored-by: M <m@e.com>\n"
	result := appendCoauthorTrailers(orig, []git.RemoteCommit{
		{Author: "M", AuthorEmail: "m@e.com"},
	})
	require.Equal(t, 1, strings.Count(result, "Co-authored-by: M <m@e.com>"))
}

func TestAppendCoauthorTrailers_FallbackEmailWhenMissing(t *testing.T) {
	orig := "subject\n\ncommit-id: aaaaaaaa\n"
	result := appendCoauthorTrailers(orig, []git.RemoteCommit{
		{Author: "M", AuthorEmail: ""},
	})
	require.Contains(t, result, "Co-authored-by: M <noreply@example.com>")
}

func TestAppendCoauthorTrailers_EmptyAuthorIgnored(t *testing.T) {
	orig := "subject\n\ncommit-id: aaaaaaaa\n"
	result := appendCoauthorTrailers(orig, []git.RemoteCommit{
		{Author: "", AuthorEmail: "anon@e.com"},
	})
	require.NotContains(t, result, "Co-authored-by:")
}

func TestApplyDivergencePolicy_Merge_AnyRefused_RefusesUpdate(t *testing.T) {
	sd, mock, out, _ := stubFoldStackedDiff(t)

	mock.ExpectRevParseHead("orig-head-sha")
	mock.ExpectLogTargetMessage("sha-A", "subject\n\ncommit-id: aaaaaaaa")
	mock.ExpectCheckoutDetach("sha-A")
	mock.ExpectCherryPickNoCommit("sha-M1")
	mock.ExpectCommitAmendWithMessage(testFoldMsgPath)
	mock.ExpectRevParseHead("new-target-sha")
	mock.ExpectRebaseOnto("new-target-sha", "sha-A", "orig-head-sha")

	divs := []git.Divergence{
		{
			HeadBranch:     "spr/master/aaaaaaaa",
			LocalCommit:    git.Commit{CommitID: "aaaaaaaa", CommitHash: "sha-A"},
			ForeignCommits: []git.RemoteCommit{foreignCommit("sha-M1", "fix", "M", "m@e.com")},
			Reason:         git.DivergenceForeignCommits,
		},
		{
			HeadBranch:  "spr/master/bbbbbbbb",
			LocalCommit: git.Commit{CommitID: "bbbbbbbb", CommitHash: "sha-B"},
			MismatchCID: "99999999",
			Reason:      git.DivergenceCidMismatch,
		},
	}
	ok := sd.applyDivergencePolicy(context.Background(), nil, divs)
	require.False(t, ok, "anything refused → refuse update")
	require.Contains(t, out.String(), "Folded 1")
	require.Contains(t, out.String(), "Refused")
	require.Contains(t, out.String(), "Refusing to continue")
	mock.ExpectationsMet()
}
