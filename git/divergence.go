package git

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxDivergenceWalkDepth caps how far back from a remote head we walk
// looking for our commit-id trailer. A maintainer who pushed more than this
// many commits is treated as a deep rewrite (DivergenceCidNotFound).
const MaxDivergenceWalkDepth = 20

// RemoteHead is what we know about a remote PR head ref after fetching it.
// History is the walk back from the tip: History[0] is the tip itself,
// History[1] its parent, and so on, up to MaxDivergenceWalkDepth entries.
type RemoteHead struct {
	Branch  string
	TipSHA  string
	History []RemoteCommit
}

// RemoteCommit is one commit on the walk back from a remote head, with the
// minimum metadata needed by detection. CommitID is the value of the
// `commit-id:` trailer if present in the body, "" otherwise.
type RemoteCommit struct {
	SHA      string
	Subject  string
	Author   string
	Body     string
	CommitID string
}

// DivergenceReason categorises why a PR head differs from the local commit.
type DivergenceReason int

const (
	// DivergenceForeignCommits — the remote head has one or more commits
	// stacked on top of a commit that does carry our matching commit-id
	// trailer. The extras are very likely a maintainer's contribution.
	DivergenceForeignCommits DivergenceReason = iota

	// DivergenceCidMismatch — the remote head tip (or a commit between
	// the tip and where we expected ours to be) carries a commit-id
	// trailer that doesn't match ours. Usually a hand-edit or a
	// cherry-pick from elsewhere; needs human review.
	DivergenceCidMismatch

	// DivergenceCidNotFound — walking back up to MaxDivergenceWalkDepth
	// commits never found a commit carrying our commit-id trailer. The
	// remote branch has likely been rewritten beyond recognition.
	DivergenceCidNotFound
)

func (r DivergenceReason) String() string {
	switch r {
	case DivergenceForeignCommits:
		return "ForeignCommits"
	case DivergenceCidMismatch:
		return "CidMismatch"
	case DivergenceCidNotFound:
		return "CidNotFound"
	default:
		return "Unknown"
	}
}

// Divergence describes one PR head that has drifted from the local stack
// in a way that needs the user's attention. ForeignCommits is populated
// only for Reason == DivergenceForeignCommits.
type Divergence struct {
	HeadBranch     string
	LocalCommit    Commit
	RemoteTipSHA   string
	ForeignCommits []RemoteCommit
	MismatchCID    string // set when Reason == DivergenceCidMismatch
	Reason         DivergenceReason
}

// commitIDTrailerRegex matches the `commit-id:` trailer that spr appends
// to every commit it manages. It's deliberately permissive about the
// surrounding whitespace and case to tolerate hand-edits.
var commitIDTrailerRegex = regexp.MustCompile(`(?mi)^\s*commit-id:\s*([a-f0-9]{8})\s*$`)

// ExtractCommitIDTrailer returns the value of the `commit-id:` trailer if
// present in the commit body, otherwise the empty string. When multiple
// trailers appear (rare) the LAST one wins, matching spr's own append
// behavior.
func ExtractCommitIDTrailer(body string) string {
	matches := commitIDTrailerRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

// DetectDivergence inspects each local commit's matching remote PR head
// branch and reports any divergence that requires user attention.
//
// `remoteHeads` is keyed by the full branch name (e.g. "spr/master/cc11dd22").
// A local commit whose corresponding branch is absent from the map is
// treated as "PR not yet pushed" and skipped silently.
//
// The returned slice is in the same order as `localCommits`; PRs without
// divergence are omitted. An empty slice means everything is in sync (or
// the only differences look like normal local amends).
func DetectDivergence(localCommits []Commit, branchPrefix, targetBranch string,
	remoteHeads map[string]RemoteHead, maxWalkDepth int) []Divergence {

	if maxWalkDepth <= 0 {
		maxWalkDepth = MaxDivergenceWalkDepth
	}

	var out []Divergence
	for _, local := range localCommits {
		if local.CommitID == "" {
			continue
		}
		branch := branchPrefix + "/" + targetBranch + "/" + local.CommitID
		head, ok := remoteHeads[branch]
		if !ok {
			continue
		}
		if head.TipSHA == "" || head.TipSHA == local.CommitHash {
			continue
		}

		d, divergent := classifyHead(local, branch, head, maxWalkDepth)
		if divergent {
			out = append(out, d)
		}
	}
	return out
}

// classifyHead implements the per-head walk + decision logic. Returns the
// Divergence record and whether it should be reported (false for the
// "amended in place" case, which the caller skips).
func classifyHead(local Commit, branch string, head RemoteHead, maxWalkDepth int) (Divergence, bool) {
	limit := len(head.History)
	if limit > maxWalkDepth {
		limit = maxWalkDepth
	}

	for i := 0; i < limit; i++ {
		c := head.History[i]
		switch {
		case c.CommitID == local.CommitID:
			if i == 0 {
				// Tip carries our trailer but SHA differs from local.
				// Stage 1 interprets this as a local amend and proceeds.
				return Divergence{}, false
			}
			return Divergence{
				HeadBranch:     branch,
				LocalCommit:    local,
				RemoteTipSHA:   head.TipSHA,
				ForeignCommits: append([]RemoteCommit{}, head.History[:i]...),
				Reason:         DivergenceForeignCommits,
			}, true

		case c.CommitID != "":
			// Hit a different commit-id trailer before finding ours.
			return Divergence{
				HeadBranch:   branch,
				LocalCommit:  local,
				RemoteTipSHA: head.TipSHA,
				MismatchCID:  c.CommitID,
				Reason:       DivergenceCidMismatch,
			}, true
		}
	}

	return Divergence{
		HeadBranch:   branch,
		LocalCommit:  local,
		RemoteTipSHA: head.TipSHA,
		Reason:       DivergenceCidNotFound,
	}, true
}

// ReadRemoteHeads fetches the named branches from `remote` (with explicit
// refspecs, so a restrictive remote.origin.fetch can't disable detection)
// and returns one RemoteHead per branch that exists on the remote, keyed
// by branch name. Branches absent from the remote are silently omitted.
//
// At most maxWalkDepth commits are walked back from each tip. Pass 0 to
// use MaxDivergenceWalkDepth.
func ReadRemoteHeads(gitcmd GitInterface, remote string, branches []string,
	maxWalkDepth int) (map[string]RemoteHead, error) {

	if len(branches) == 0 {
		return nil, nil
	}
	if maxWalkDepth <= 0 {
		maxWalkDepth = MaxDivergenceWalkDepth
	}

	refspecs := make([]string, len(branches))
	for i, b := range branches {
		refspecs[i] = b + ":refs/remotes/" + remote + "/" + b
	}
	fetchArgs := "fetch " + remote + " " + strings.Join(refspecs, " ")
	// Ignore the error: a missing ref on the remote is a normal "PR not
	// yet pushed" case. Per-branch readSingleHead handles absence below.
	_ = gitcmd.Git(fetchArgs, nil)

	out := make(map[string]RemoteHead, len(branches))
	for _, b := range branches {
		ref := "refs/remotes/" + remote + "/" + b
		head, ok := readSingleHead(gitcmd, b, ref, maxWalkDepth)
		if ok {
			out[b] = head
		}
	}
	return out, nil
}

// logSeparator is what we put in the git log --format= string. We rely on
// `git log -z` to separate commits with NUL bytes; within each commit, %n
// (newline) separates the SHA, author, subject, and body fields. The body
// (which can contain embedded newlines) is always last so a SplitN(...,4)
// captures it as the final chunk.
const remoteLogFormat = "%H%n%an%n%s%n%b"

func readSingleHead(gitcmd GitInterface, branch, ref string, maxWalkDepth int) (RemoteHead, bool) {
	var output string
	cmd := fmt.Sprintf("log -z --no-color --format=%s -n %d %s",
		remoteLogFormat, maxWalkDepth, ref)
	if err := gitcmd.Git(cmd, &output); err != nil {
		return RemoteHead{}, false
	}
	history := parseRemoteLog(output)
	if len(history) == 0 {
		return RemoteHead{}, false
	}
	return RemoteHead{
		Branch:  branch,
		TipSHA:  history[0].SHA,
		History: history,
	}, true
}

// parseRemoteLog decodes the output of `git log -z --format=remoteLogFormat`.
// Each commit chunk is NUL-terminated; within a chunk, four fields are
// newline-separated: SHA, author, subject, body. Body may contain newlines.
func parseRemoteLog(s string) []RemoteCommit {
	if s == "" {
		return nil
	}
	var out []RemoteCommit
	for _, entry := range strings.Split(s, "\x00") {
		entry = strings.TrimLeft(entry, "\n")
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "\n", 4)
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		body := strings.TrimRight(parts[3], "\n")
		out = append(out, RemoteCommit{
			SHA:      parts[0],
			Author:   parts[1],
			Subject:  parts[2],
			Body:     body,
			CommitID: ExtractCommitIDTrailer(body),
		})
	}
	return out
}
