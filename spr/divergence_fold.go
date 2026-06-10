package spr

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

// foldDivergences implements the `onRemoteDivergence: merge` policy.
//
// For each Divergence whose Reason is DivergenceForeignCommits the
// maintainer's commits are cherry-picked onto the matching local commit's
// tree, the commit is amended in place (preserving its message, including
// the `commit-id:` trailer), and the rest of the stack is re-applied on
// top via `git rebase --onto`. Non-foldable reasons (CidMismatch,
// CidNotFound) and divergences whose cherry-pick conflicts with the local
// stack are returned as `refused` for the caller to surface.
//
// Divergences are processed top-down so that lower-stack targets are not
// affected by upstream re-stacking — their SHAs (as captured at detection
// time) remain valid when their turn comes.
//
// On a successful fold the working tree's HEAD points at the new top of
// the re-stacked stack; the next `spr update` push publishes it.
// On per-divergence failure the partial state is rolled back to the
// pre-fold HEAD before the next divergence is attempted.
func (sd *stackediff) foldDivergences(divs []git.Divergence) (folded, refused []git.Divergence) {
	var foldable []git.Divergence
	for _, d := range divs {
		if d.Reason == git.DivergenceForeignCommits {
			foldable = append(foldable, d)
		} else {
			refused = append(refused, d)
		}
	}
	if len(foldable) == 0 {
		return nil, refused
	}

	// Top-down: process the highest divergence in the stack first so that
	// lower-stack targets remain stable. DetectDivergence returns
	// divergences in bottom-first order, so iterate in reverse.
	for i := len(foldable) - 1; i >= 0; i-- {
		d := foldable[i]
		if err := sd.foldOneDivergence(d); err != nil {
			refused = append(refused, d)
			fmt.Fprintf(sd.output, "warning: fold of %s deferred: %v\n", d.HeadBranch, err)
			continue
		}
		folded = append(folded, d)
	}
	return folded, refused
}

// foldOneDivergence performs a single per-divergence fold atomically. On
// any failure between steps it rolls HEAD back to its original position
// before returning the error.
//
// Sequence (each step is one git call):
//  1. capture origHead       (`rev-parse HEAD`)
//  2. read target message    (`log -1 --format=%B <target>`)
//  3. detach at target       (`checkout --detach <target>`)
//  4. apply each foreign diff (`cherry-pick --no-commit <sha>`, oldest-first)
//  5. amend target with new tree + co-author trailers
//     (`commit --amend -F <msgfile>`)
//  6. capture newTarget       (`rev-parse HEAD`)
//  7. re-stack descendants    (`rebase --onto <newTarget> <target> <origHead>`)
func (sd *stackediff) foldOneDivergence(d git.Divergence) error {
	targetHash := d.LocalCommit.CommitHash
	if targetHash == "" {
		return fmt.Errorf("local commit hash not available for commit-id %s", d.LocalCommit.CommitID)
	}

	var origHead string
	if err := sd.gitcmd.Git("rev-parse HEAD", &origHead); err != nil {
		return fmt.Errorf("rev-parse HEAD: %v", err)
	}
	origHead = strings.TrimSpace(origHead)

	// Read target's current message so we can preserve it (with the
	// commit-id trailer) and append Co-authored-by trailers.
	var targetMessage string
	if err := sd.gitcmd.Git("log -1 --format=%B "+targetHash, &targetMessage); err != nil {
		return fmt.Errorf("log target message: %v", err)
	}

	newMessage := appendCoauthorTrailers(targetMessage, d.ForeignCommits)
	msgPath, cleanup, err := sd.writeFoldMessage(newMessage)
	if err != nil {
		return fmt.Errorf("write fold message: %v", err)
	}
	defer cleanup()

	if err := sd.gitcmd.Git("checkout --detach "+targetHash, nil); err != nil {
		return fmt.Errorf("checkout target failed: %v", err)
	}

	// Foreign commits are tip-first in d.ForeignCommits; apply oldest
	// first so each cherry-pick is incremental.
	for i := len(d.ForeignCommits) - 1; i >= 0; i-- {
		fc := d.ForeignCommits[i]
		if err := sd.gitcmd.Git("cherry-pick --no-commit "+fc.SHA, nil); err != nil {
			_ = sd.gitcmd.Git("cherry-pick --abort", nil)
			_ = sd.gitcmd.Git("checkout "+origHead, nil)
			return fmt.Errorf("cherry-pick of %s conflicts with local stack", shortSHA(fc.SHA))
		}
	}

	// Amend in place with the new message file. The message preserves
	// the target's body + commit-id trailer and appends one
	// `Co-authored-by:` line per unique maintainer.
	if err := sd.gitcmd.Git("commit --amend -F "+msgPath, nil); err != nil {
		_ = sd.gitcmd.Git("checkout "+origHead, nil)
		return fmt.Errorf("commit --amend: %v", err)
	}

	var newTarget string
	if err := sd.gitcmd.Git("rev-parse HEAD", &newTarget); err != nil {
		_ = sd.gitcmd.Git("checkout "+origHead, nil)
		return fmt.Errorf("rev-parse new HEAD: %v", err)
	}
	newTarget = strings.TrimSpace(newTarget)

	// Re-stack: commits between targetHash (exclusive) and origHead get
	// re-applied on top of newTarget. For a fold of the stack tip the
	// range is empty and rebase --onto just resets HEAD to newTarget.
	rebaseCmd := fmt.Sprintf("rebase --onto %s %s %s", newTarget, targetHash, origHead)
	if err := sd.gitcmd.Git(rebaseCmd, nil); err != nil {
		_ = sd.gitcmd.Git("rebase --abort", nil)
		_ = sd.gitcmd.Git("checkout "+origHead, nil)
		return fmt.Errorf("rebase --onto failed: %v", err)
	}
	return nil
}

// appendCoauthorTrailers returns origMessage with one
// `Co-authored-by: Name <email>` trailer per unique (name, email) pair
// found in foreigns. Already-present trailers are not re-added. Maintains
// at least one blank line between the body and the trailer block, since
// git's trailer parser requires that separation.
func appendCoauthorTrailers(origMessage string, foreigns []git.RemoteCommit) string {
	trimmed := strings.TrimRight(origMessage, "\n")
	seen := make(map[string]bool)
	// Pre-seed with any trailers already in the message so re-folds don't
	// duplicate them.
	for _, line := range strings.Split(trimmed, "\n") {
		if strings.HasPrefix(strings.ToLower(line), "co-authored-by:") {
			seen[strings.TrimSpace(line)] = true
		}
	}
	var trailers []string
	for _, fc := range foreigns {
		if fc.Author == "" {
			continue
		}
		email := fc.AuthorEmail
		if email == "" {
			email = "noreply@example.com"
		}
		trailer := fmt.Sprintf("Co-authored-by: %s <%s>", fc.Author, email)
		if seen[trailer] {
			continue
		}
		seen[trailer] = true
		trailers = append(trailers, trailer)
	}
	if len(trailers) == 0 {
		return trimmed + "\n"
	}
	// If the message ends with a trailer block (no blank line before),
	// append directly. If it ends with prose (no trailing trailer line),
	// insert one blank line.
	if !endsWithTrailerBlock(trimmed) {
		trimmed += "\n"
	}
	return trimmed + "\n" + strings.Join(trailers, "\n") + "\n"
}

// endsWithTrailerBlock reports whether the last non-blank line of msg
// looks like a git trailer (Token: value). Used to decide whether to
// insert a blank-line separator before appending new trailers.
func endsWithTrailerBlock(msg string) bool {
	lines := strings.Split(msg, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// A trailer line is one with a "Token: value" shape and no spaces
		// in the token. spr's own `commit-id:` lines match this.
		colonIdx := strings.Index(line, ":")
		if colonIdx <= 0 {
			return false
		}
		token := line[:colonIdx]
		if strings.ContainsAny(token, " \t") {
			return false
		}
		return true
	}
	return false
}

// writeFoldMessage materialises the given content as a file that
// `git commit --amend -F` can read. Production uses os.CreateTemp; tests
// inject sd.foldMessageWriter to return a fixed path so mock expectations
// stay deterministic.
func (sd *stackediff) writeFoldMessage(content string) (string, func(), error) {
	if sd.foldMessageWriter != nil {
		return sd.foldMessageWriter(content)
	}
	f, err := os.CreateTemp("", "spr-fold-msg-*.txt")
	if err != nil {
		return "", nil, err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// postFoldComments posts one PR comment per folded divergence, telling
// the maintainer their commits were absorbed into the underlying local
// commit and warning against `git push --force` on the head branch.
// Skipped silently when info is nil or no matching PR is found.
func (sd *stackediff) postFoldComments(ctx context.Context, info *github.GitHubInfo, folded []git.Divergence) {
	if info == nil {
		return
	}
	for _, d := range folded {
		pr := findPRByCommitID(info.PullRequests, d.LocalCommit.CommitID)
		if pr == nil {
			continue
		}
		sd.github.CommentPullRequest(ctx, pr, buildFoldComment(d))
	}
}

func findPRByCommitID(prs []*github.PullRequest, cid string) *github.PullRequest {
	for _, pr := range prs {
		if pr.Commit.CommitID == cid {
			return pr
		}
	}
	return nil
}

func buildFoldComment(d git.Divergence) string {
	var b strings.Builder
	fmt.Fprintf(&b, "spr integrated %d commit(s) from this branch into the underlying local commit "+
		"(commit-id `%s`):\n\n", len(d.ForeignCommits), d.LocalCommit.CommitID)
	for _, fc := range d.ForeignCommits {
		fmt.Fprintf(&b, "- `%s` %s — %s\n", shortSHA(fc.SHA), fc.Subject, fc.Author)
	}
	fmt.Fprintf(&b, "\nThe head branch was force-pushed to reflect this. If you have local "+
		"changes on top, please:\n\n")
	fmt.Fprintf(&b, "```bash\ngit fetch && git reset --hard origin/%s\n```\n\n", d.HeadBranch)
	fmt.Fprintf(&b, "Do **not** `git push --force` — it would undo the integration and "+
		"the contributor's next `spr update` would absorb your work again.\n")
	return b.String()
}

// printFoldSummary prints one line per PR explaining what happened.
// Called whether or not the fold succeeded — the caller decides what to
// do next based on the refused list.
func printFoldSummary(w io.Writer, folded, refused []git.Divergence) {
	if len(folded) > 0 {
		fmt.Fprintf(w, "Folded %d PR head(s) into local stack:\n", len(folded))
		for _, d := range folded {
			fmt.Fprintf(w, "  - %s: %d commit(s) absorbed\n",
				d.HeadBranch, len(d.ForeignCommits))
		}
	}
	if len(refused) > 0 {
		fmt.Fprintf(w, "Refused %d PR head(s) (needs manual resolution):\n", len(refused))
		for _, d := range refused {
			fmt.Fprintf(w, "  - %s: %s\n", d.HeadBranch, describeDivergence(d))
		}
	}
}
