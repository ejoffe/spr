package spr

import (
	"fmt"
	"io"
	"strings"

	"github.com/ejoffe/spr/git"
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
//  1. capture origHead   (`rev-parse HEAD`)
//  2. detach at target   (`checkout --detach <target>`)
//  3. apply each foreign diff   (`cherry-pick --no-commit <sha>`, oldest first)
//  4. amend target with new tree   (`commit --amend --no-edit`)
//  5. capture newTarget   (`rev-parse HEAD`)
//  6. re-stack descendants   (`rebase --onto <newTarget> <target> <origHead>`)
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

	if err := sd.gitcmd.Git("checkout --detach "+targetHash, nil); err != nil {
		// Couldn't even detach — nothing to roll back beyond ensuring
		// HEAD is at origHead, which it already is.
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

	// Amend in place: keeps the target's message (and commit-id trailer)
	// but the tree now includes the foreign diffs we just cherry-picked.
	// Stack 3/3 will replace --no-edit with a custom message file that
	// appends Co-authored-by trailers.
	if err := sd.gitcmd.Git("commit --amend --no-edit", nil); err != nil {
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
