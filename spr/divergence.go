package spr

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"golang.org/x/term"
)

// DivergenceAction is the user's resolution choice when remote-side
// changes are detected on PR head branches.
type DivergenceAction int

const (
	// DivergenceActionDrop: force-push local, wiping the remote commits.
	// Matches pre-fix behavior; selected by config="drop" or interactive [d].
	DivergenceActionDrop DivergenceAction = iota
	// DivergenceActionAbort: stop `spr update` so the user can resolve manually.
	DivergenceActionAbort
)

// checkRemoteDivergence inspects every PR head branch for commits the
// local stack doesn't carry (typically a maintainer's suggestion pushed
// via "Allow edits by maintainers"). When found, applies the configured
// onRemoteDivergence policy.
//
// Returns true to proceed, false to abort. Any user-facing message has
// already been printed. localCommits is the already-fetched local stack
// (avoids a redundant `git log` call vs. re-fetching here).
func (sd *stackediff) checkRemoteDivergence(ctx context.Context, localCommits []git.Commit) bool {
	if len(localCommits) == 0 {
		return true
	}
	var branches []string
	for _, c := range localCommits {
		if c.CommitID == "" {
			continue
		}
		branches = append(branches, git.BranchNameFromCommit(sd.config, c))
	}
	if len(branches) == 0 {
		return true
	}

	heads, err := git.ReadRemoteHeads(sd.gitcmd, sd.config.Repo.GitHubRemote,
		branches, git.MaxDivergenceWalkDepth)
	if err != nil {
		// Fail open: a fetch error shouldn't block update. Worst case
		// the legacy force-push behavior happens (same as before this fix).
		fmt.Fprintf(sd.output, "warning: remote head fetch failed: %v\n", err)
		return true
	}

	divergences := git.DetectDivergence(localCommits, sd.config.User.BranchPrefix,
		sd.config.Repo.GitHubBranch, heads, git.MaxDivergenceWalkDepth)
	if len(divergences) == 0 {
		return true
	}
	return sd.applyDivergencePolicy(divergences)
}

// applyDivergencePolicy routes a non-empty Divergence list through the
// configured policy. Exposed as a method (not a free function) so tests
// can inject sd.divergencePromptFn.
func (sd *stackediff) applyDivergencePolicy(divergences []git.Divergence) bool {
	policy := sd.config.Repo.OnRemoteDivergence
	if policy == "" {
		policy = config.OnRemoteDivergenceAsk
	}

	switch policy {
	case config.OnRemoteDivergenceDrop:
		printDropSummary(sd.output, divergences)
		return true

	case config.OnRemoteDivergenceMerge:
		// Stage 2 not implemented; fall through to interactive ask.
		fmt.Fprintf(sd.output,
			"note: onRemoteDivergence=merge is reserved for a Stage 2 follow-up;\n"+
				"      falling back to interactive prompt for this run.\n")
		fallthrough

	case config.OnRemoteDivergenceAsk:
		return sd.promptDivergenceAction(divergences) == DivergenceActionDrop

	default:
		fmt.Fprintf(sd.output,
			"error: invalid onRemoteDivergence value %q (expected ask, drop, or merge)\n",
			policy)
		return false
	}
}

// promptDivergenceAction dispatches to the test override (if any), else
// to the interactive prompt on TTY, else to the non-TTY refusal path.
func (sd *stackediff) promptDivergenceAction(divergences []git.Divergence) DivergenceAction {
	if sd.divergencePromptFn != nil {
		return sd.divergencePromptFn(divergences)
	}
	if isTTY(sd.input) {
		return interactiveDivergencePrompt(sd.input, sd.output, divergences)
	}
	printResolutionGuide(sd.output, divergences)
	fmt.Fprintf(sd.output,
		"\nstdin is not a terminal; refusing to drop remote commits silently.\n"+
			"Resolve the divergence(s) above manually, or set onRemoteDivergence: drop in .spr.yml to opt in.\n")
	return DivergenceActionAbort
}

func interactiveDivergencePrompt(in io.Reader, out io.Writer, divergences []git.Divergence) DivergenceAction {
	printResolutionGuide(out, divergences)
	fmt.Fprintf(out, "\n[d]rop remote commits (force-push local), [a]bort? ")
	reader := bufio.NewReader(in)
	line, _ := reader.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "d", "drop":
		return DivergenceActionDrop
	default:
		return DivergenceActionAbort
	}
}

// printDropSummary prints a one-line warning per PR when the user has
// explicitly opted into the "drop" policy.
func printDropSummary(w io.Writer, divergences []git.Divergence) {
	fmt.Fprintf(w, "warning: dropping %d remote-side change(s) per onRemoteDivergence=drop:\n",
		len(divergences))
	for _, d := range divergences {
		fmt.Fprintf(w, "  - %s: %s\n", d.HeadBranch, describeDivergence(d))
	}
}

// printResolutionGuide prints the full per-PR guidance, including
// copy-pasteable git commands. Used both for the non-TTY refusal and as
// part of the interactive prompt header.
func printResolutionGuide(w io.Writer, divergences []git.Divergence) {
	fmt.Fprintf(w, "\n%d pull request head branch(es) have changes on the remote that your local stack doesn't:\n\n",
		len(divergences))
	for _, d := range divergences {
		printOneResolution(w, d)
	}
}

func printOneResolution(w io.Writer, d git.Divergence) {
	cid := d.LocalCommit.CommitID
	fmt.Fprintf(w, "  %s  (%s)\n", d.HeadBranch, describeDivergence(d))

	switch d.Reason {
	case git.DivergenceForeignCommits:
		for _, c := range d.ForeignCommits {
			fmt.Fprintf(w, "      %s  %s  (%s)\n",
				shortSHA(c.SHA), c.Subject, c.Author)
		}
		fmt.Fprintf(w, "\n    To fold them into your local commit (preserves commit-id %s):\n", cid)
		fmt.Fprintf(w, "      git fetch origin %s\n", d.HeadBranch)
		fmt.Fprintf(w, "      git rebase -i origin/<target>   # mark commit-id %s as 'edit'\n", cid)
		fmt.Fprintf(w, "      git cherry-pick FETCH_HEAD~%d..FETCH_HEAD\n", len(d.ForeignCommits))
		fmt.Fprintf(w, "      git commit --amend --no-edit\n")
		fmt.Fprintf(w, "      git rebase --continue\n")
		fmt.Fprintf(w, "      git spr update\n\n")

	case git.DivergenceCidMismatch:
		fmt.Fprintf(w, "    Remote head tip carries commit-id %s instead of expected %s.\n",
			d.MismatchCID, cid)
		fmt.Fprintf(w, "    Likely a hand-edit or cherry-pick from elsewhere. Inspect manually:\n")
		fmt.Fprintf(w, "      git fetch origin %s\n", d.HeadBranch)
		fmt.Fprintf(w, "      git log origin/%s\n\n", d.HeadBranch)

	case git.DivergenceCidNotFound:
		fmt.Fprintf(w, "    Remote head doesn't carry your commit-id %s in the last %d commits.\n",
			cid, git.MaxDivergenceWalkDepth)
		fmt.Fprintf(w, "    Likely a deep rewrite of the branch. Inspect manually:\n")
		fmt.Fprintf(w, "      git fetch origin %s\n", d.HeadBranch)
		fmt.Fprintf(w, "      git log origin/%s\n\n", d.HeadBranch)
	}
}

func describeDivergence(d git.Divergence) string {
	switch d.Reason {
	case git.DivergenceForeignCommits:
		return fmt.Sprintf("%d foreign commit(s) on remote", len(d.ForeignCommits))
	case git.DivergenceCidMismatch:
		return fmt.Sprintf("unexpected commit-id %s at remote tip", d.MismatchCID)
	case git.DivergenceCidNotFound:
		return fmt.Sprintf("commit-id %s missing within last %d remote commits",
			d.LocalCommit.CommitID, git.MaxDivergenceWalkDepth)
	default:
		return "unknown divergence"
	}
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// isTTY returns true when r is a *os.File backed by a real terminal.
// Tests substituting bytes.Buffer (or any non-*os.File) get false, which
// routes them through the non-TTY refusal path unless they also set
// stackediff.divergencePromptFn.
func isTTY(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
