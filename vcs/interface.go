package vcs

import (
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

// VCSOperations abstracts the version control operations that differ between
// git and jj (Jujutsu). Operations like push, fetch, and branch management
// stay on git.GitInterface; only history-rewriting operations are abstracted here.
type VCSOperations interface {
	// FetchAndRebase fetches from remote and rebases local stack onto updated trunk.
	// Git: git fetch + git rebase origin/main --autostash
	// jj:  jj git fetch + jj rebase -b @ -d main@origin
	FetchAndRebase(cfg *config.Config) error

	// GetLocalCommitStack returns unmerged commits (bottom-first), adding
	// commit-id trailers if missing.
	// Git: git log origin/main..HEAD, then git rebase -i with spr_reword_helper if needed
	// jj:  jj log -r 'trunk()..@' --reversed, then jj describe for missing trailers
	GetLocalCommitStack(cfg *config.Config, gitcmd git.GitInterface) []git.Commit

	// AmendInto squashes working copy changes into a specific commit in the stack.
	// Git: git commit --fixup <hash> + git rebase -i --autosquash --autostash
	// jj:  jj squash --into <change-id>
	AmendInto(commit git.Commit) error

	// EditStart checks out a commit for editing (interactive edit session).
	// Git: git rebase -i with 'edit' stop
	// jj:  jj edit <change-id>
	EditStart(commit git.Commit) error

	// EditFinish completes an edit session.
	// Git: git add -A + git commit --amend --no-edit + git rebase --continue
	// jj:  jj new <original-@> (changes are auto-captured)
	EditFinish() error

	// EditAbort cancels an edit session.
	// Git: git rebase --abort
	// jj:  jj op restore <saved-op-id>
	EditAbort() error

	// PrepareForPush saves working state before push and returns a cleanup func.
	// Git: git stash / git stash pop
	// jj:  no-op (working copy is always a commit)
	PrepareForPush() (cleanup func(), err error)

	// IsEditing returns true if an edit session is in progress.
	IsEditing() bool

	// EditStatePath returns the path to the edit state file.
	EditStatePath() string

	// CheckStackCompleteness checks whether the current working copy position
	// might cause spr to see an incomplete stack. Returns a non-empty warning
	// string if there are commits that would be excluded (e.g. @ has descendants
	// in jj, or HEAD is detached in git). Returns "" if everything looks fine.
	CheckStackCompleteness() string
}

// NewVCSOperations creates a VCSOperations implementation appropriate for the
// current repository. If a .jj/ directory exists (jj-colocated repo) and
// the user has not set noJJ, returns a jj implementation.
// Otherwise returns a git implementation.
func NewVCSOperations(cfg *config.Config, gitcmd git.GitInterface) VCSOperations {
	if !cfg.User.NoJJ && IsJJColocated(gitcmd.RootDir()) {
		return NewJjOps(cfg, NewJjCmd(gitcmd.RootDir()), gitcmd)
	}
	return NewGitOps(cfg, gitcmd)
}
