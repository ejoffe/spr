package git

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// helper: build a RemoteHead with a single tip-and-walk history.
// shas[0] is the tip; shas[len-1] is the oldest ancestor we know about.
// cids parallels shas: cids[i] is the commit-id trailer for shas[i] ("" if none).
func newRemoteHead(branch string, shas, cids []string) RemoteHead {
	if len(shas) != len(cids) {
		panic("shas and cids must be the same length")
	}
	history := make([]RemoteCommit, len(shas))
	for i := range shas {
		history[i] = RemoteCommit{SHA: shas[i], CommitID: cids[i]}
	}
	return RemoteHead{
		Branch:  branch,
		TipSHA:  shas[0],
		History: history,
	}
}

func TestDetectDivergence_Empty(t *testing.T) {
	got := DetectDivergence(nil, "spr", "master", nil, MaxDivergenceWalkDepth)
	require.Empty(t, got, "no commits → no divergence")
}

func TestDetectDivergence_NoLocalCommitID(t *testing.T) {
	// Local commit without commit-id trailer (e.g. WIP) — silently skipped.
	local := []Commit{{CommitHash: "deadbeef", CommitID: ""}}
	got := DetectDivergence(local, "spr", "master", nil, MaxDivergenceWalkDepth)
	require.Empty(t, got)
}

func TestDetectDivergence_NewPRNotYetPushed(t *testing.T) {
	// Local commit with a cid, but the corresponding branch isn't in
	// remoteHeads — first-time PR.
	local := []Commit{{CommitHash: "deadbeef", CommitID: "11111111"}}
	got := DetectDivergence(local, "spr", "master", map[string]RemoteHead{}, MaxDivergenceWalkDepth)
	require.Empty(t, got, "missing remote head must skip silently")
}

func TestDetectDivergence_SHAsMatch(t *testing.T) {
	// In sync: local SHA == remote tip SHA. No divergence.
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111",
			[]string{"sha-A"}, []string{"11111111"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Empty(t, got)
}

func TestDetectDivergence_AmendedInPlace(t *testing.T) {
	// Tip carries our trailer but the SHA differs (local user amended).
	// Stage 1 treats this as a normal local amend — no divergence reported.
	local := []Commit{{CommitHash: "sha-A-new", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111",
			[]string{"sha-A-old"}, []string{"11111111"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Empty(t, got, "amend-in-place must not flag — indistinguishable from user amend")
}

func TestDetectDivergence_ForeignCommitOnTop(t *testing.T) {
	// One maintainer commit M sitting above our commit A on the head branch.
	// Walk: [M (no cid), A (cid 11111111)]. Expect ForeignCommits=[M].
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111",
			[]string{"sha-M", "sha-A"},
			[]string{"", "11111111"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Len(t, got, 1)
	require.Equal(t, DivergenceForeignCommits, got[0].Reason)
	require.Equal(t, "spr/master/11111111", got[0].HeadBranch)
	require.Equal(t, "sha-M", got[0].RemoteTipSHA)
	require.Len(t, got[0].ForeignCommits, 1)
	require.Equal(t, "sha-M", got[0].ForeignCommits[0].SHA)
}

func TestDetectDivergence_MultipleForeignCommits(t *testing.T) {
	// Two maintainer commits above ours.
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111",
			[]string{"sha-M2", "sha-M1", "sha-A"},
			[]string{"", "", "11111111"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Len(t, got, 1)
	require.Equal(t, DivergenceForeignCommits, got[0].Reason)
	require.Len(t, got[0].ForeignCommits, 2)
	require.Equal(t, "sha-M2", got[0].ForeignCommits[0].SHA)
	require.Equal(t, "sha-M1", got[0].ForeignCommits[1].SHA)
}

func TestDetectDivergence_MidStackSideBranch(t *testing.T) {
	// Local: A → B → C, stack of 3. Maintainer adds M on top of PR#2's
	// head branch (spr/master/bbbbbbbb). M's SHA is on a SIDE BRANCH
	// from the local-stack perspective — C and M are siblings parented
	// by B. Detection should flag PR#2 only.
	local := []Commit{
		{CommitHash: "sha-A", CommitID: "aaaaaaaa"},
		{CommitHash: "sha-B", CommitID: "bbbbbbbb"},
		{CommitHash: "sha-C", CommitID: "cccccccc"},
	}
	heads := map[string]RemoteHead{
		"spr/master/aaaaaaaa": newRemoteHead("spr/master/aaaaaaaa",
			[]string{"sha-A"}, []string{"aaaaaaaa"}),
		"spr/master/bbbbbbbb": newRemoteHead("spr/master/bbbbbbbb",
			[]string{"sha-M", "sha-B"},
			[]string{"", "bbbbbbbb"}),
		"spr/master/cccccccc": newRemoteHead("spr/master/cccccccc",
			[]string{"sha-C"}, []string{"cccccccc"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Len(t, got, 1, "only mid-stack PR#2 should be flagged")
	require.Equal(t, "spr/master/bbbbbbbb", got[0].HeadBranch)
	require.Equal(t, DivergenceForeignCommits, got[0].Reason)
	require.Equal(t, "bbbbbbbb", got[0].LocalCommit.CommitID)
}

func TestDetectDivergence_MultiplePRsDiverged(t *testing.T) {
	// Maintainer adds commits to both PR#1 (1 commit) and PR#3 (2 commits).
	// PR#2 untouched. Expect two divergences in stack order.
	local := []Commit{
		{CommitHash: "sha-A", CommitID: "aaaaaaaa"},
		{CommitHash: "sha-B", CommitID: "bbbbbbbb"},
		{CommitHash: "sha-C", CommitID: "cccccccc"},
	}
	heads := map[string]RemoteHead{
		"spr/master/aaaaaaaa": newRemoteHead("spr/master/aaaaaaaa",
			[]string{"sha-M1", "sha-A"},
			[]string{"", "aaaaaaaa"}),
		"spr/master/bbbbbbbb": newRemoteHead("spr/master/bbbbbbbb",
			[]string{"sha-B"}, []string{"bbbbbbbb"}),
		"spr/master/cccccccc": newRemoteHead("spr/master/cccccccc",
			[]string{"sha-M3b", "sha-M3a", "sha-C"},
			[]string{"", "", "cccccccc"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Len(t, got, 2)
	require.Equal(t, "aaaaaaaa", got[0].LocalCommit.CommitID)
	require.Len(t, got[0].ForeignCommits, 1)
	require.Equal(t, "cccccccc", got[1].LocalCommit.CommitID)
	require.Len(t, got[1].ForeignCommits, 2)
}

func TestDetectDivergence_CidMismatchAtTip(t *testing.T) {
	// Maintainer rewrote the head: tip has a trailer, but it's a
	// different cid (e.g. cherry-picked from elsewhere). Refuse.
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111",
			[]string{"sha-X"}, []string{"99999999"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, MaxDivergenceWalkDepth)
	require.Len(t, got, 1)
	require.Equal(t, DivergenceCidMismatch, got[0].Reason)
	require.Equal(t, "99999999", got[0].MismatchCID)
}

func TestDetectDivergence_CidNotFoundDeepRewrite(t *testing.T) {
	// 25 commits in history, none carry our cid, no other cid trailers
	// either — walk just hits the depth cap.
	shas := make([]string, 25)
	cids := make([]string, 25)
	for i := range shas {
		shas[i] = "rewritten-" + string(rune('a'+i))
	}
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111", shas, cids),
	}
	got := DetectDivergence(local, "spr", "master", heads, 20)
	require.Len(t, got, 1)
	require.Equal(t, DivergenceCidNotFound, got[0].Reason)
}

func TestDetectDivergence_CidNotFoundWithinDepth(t *testing.T) {
	// Our cid IS in history but BEYOND the depth limit — same outcome.
	shas := make([]string, 25)
	cids := make([]string, 25)
	for i := range shas {
		shas[i] = "rewritten-" + string(rune('a'+i))
	}
	// Put our cid at index 22, beyond the depth=20 cap.
	cids[22] = "11111111"
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111", shas, cids),
	}
	got := DetectDivergence(local, "spr", "master", heads, 20)
	require.Len(t, got, 1)
	require.Equal(t, DivergenceCidNotFound, got[0].Reason,
		"cid present but past the walk depth is reported as not found")
}

func TestDetectDivergence_CustomBranchPrefix(t *testing.T) {
	// Branch prefix configuration is honoured.
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"my-team/develop/11111111": newRemoteHead("my-team/develop/11111111",
			[]string{"sha-M", "sha-A"},
			[]string{"", "11111111"}),
	}
	got := DetectDivergence(local, "my-team", "develop", heads, MaxDivergenceWalkDepth)
	require.Len(t, got, 1)
	require.Equal(t, "my-team/develop/11111111", got[0].HeadBranch)
}

func TestDetectDivergence_DefaultDepthWhenZero(t *testing.T) {
	// Passing maxWalkDepth=0 falls back to MaxDivergenceWalkDepth.
	local := []Commit{{CommitHash: "sha-A", CommitID: "11111111"}}
	heads := map[string]RemoteHead{
		"spr/master/11111111": newRemoteHead("spr/master/11111111",
			[]string{"sha-M", "sha-A"},
			[]string{"", "11111111"}),
	}
	got := DetectDivergence(local, "spr", "master", heads, 0)
	require.Len(t, got, 1)
	require.Equal(t, DivergenceForeignCommits, got[0].Reason)
}

func TestExtractCommitIDTrailer(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"no trailer", "fix typo\n", ""},
		{"plain trailer", "subject\n\ncommit-id: abcd1234\n", "abcd1234"},
		{"trailer at very end no newline", "subject\n\ncommit-id: abcd1234", "abcd1234"},
		{"case-insensitive marker", "subject\n\nCommit-ID: abcd1234\n", "abcd1234"},
		{"leading whitespace tolerated", "subject\n\n  commit-id:   abcd1234  \n", "abcd1234"},
		{"trailer not at start of line is rejected", "see commit-id: abcd1234\n", ""},
		{"wrong length rejected", "commit-id: abc123\n", ""},
		{"non-hex rejected", "commit-id: zzzzzzzz\n", ""},
		{"multiple trailers: last wins", "commit-id: 11111111\ncommit-id: 22222222\n", "22222222"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ExtractCommitIDTrailer(tc.body))
		})
	}
}
