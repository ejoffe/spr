package vcs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

// GitOps implements VCSOperations using standard git commands.
// This is a pure extraction of the existing logic from spr.go and helpers.go.
type GitOps struct {
	cfg    *config.Config
	gitcmd git.GitInterface
}

// NewGitOps creates a git-based VCSOperations implementation.
func NewGitOps(cfg *config.Config, gitcmd git.GitInterface) *GitOps {
	return &GitOps{cfg: cfg, gitcmd: gitcmd}
}

// FetchAndRebase fetches from remote and rebases the local stack.
// Extracted from spr.go fetchAndGetGitHubInfo().
func (g *GitOps) FetchAndRebase(cfg *config.Config) error {
	if cfg.Repo.ForceFetchTags {
		g.gitcmd.MustGit("fetch --tags --force", nil)
	} else {
		g.gitcmd.MustGit("fetch", nil)
	}
	rebaseCommand := fmt.Sprintf("rebase %s/%s --autostash",
		cfg.Repo.GitHubRemote, cfg.Repo.GitHubBranch)
	return g.gitcmd.Git(rebaseCommand, nil)
}

// GetLocalCommitStack returns the local commit stack using git log.
// Delegates to the existing git.GetLocalCommitStack function.
func (g *GitOps) GetLocalCommitStack(cfg *config.Config, gitcmd git.GitInterface) []git.Commit {
	return git.GetLocalCommitStack(cfg, gitcmd)
}

// AmendInto creates a fixup commit and autosquashes it into the target.
// Extracted from spr.go AmendCommit().
func (g *GitOps) AmendInto(commit git.Commit) error {
	g.gitcmd.MustGit("commit --fixup "+commit.CommitHash, nil)
	rebaseCmd := fmt.Sprintf("rebase -i --autosquash --autostash %s/%s",
		g.cfg.Repo.GitHubRemote, g.cfg.Repo.GitHubBranch)
	g.gitcmd.MustGit(rebaseCmd, nil)
	return nil
}

// EditStart begins an interactive edit session on a commit.
// Extracted from spr.go EditCommit().
func (g *GitOps) EditStart(commit git.Commit) error {
	// Write state file
	stateContent := fmt.Sprintf("commit_id=%s\ncommit_subject=%s\n", commit.CommitID, commit.Subject)
	err := os.WriteFile(g.EditStatePath(), []byte(stateContent), 0644)
	if err != nil {
		return err
	}

	// Use the spr binary as the sequence editor to rewrite 'pick' to 'edit'
	exe, err := os.Executable()
	if err != nil {
		os.Remove(g.EditStatePath())
		return err
	}
	editorCmd := fmt.Sprintf("%s _edit-sequence %s", exe, commit.CommitHash[:7])

	rebaseCmd := fmt.Sprintf("rebase -i --autostash %s/%s",
		g.cfg.Repo.GitHubRemote, g.cfg.Repo.GitHubBranch)
	err = g.gitcmd.GitWithEditor(rebaseCmd, nil, editorCmd)
	if err != nil {
		os.Remove(g.EditStatePath())
		return err
	}
	return nil
}

// EditFinish completes an edit session by amending and continuing the rebase.
// Extracted from spr.go EditCommitDone().
func (g *GitOps) EditFinish() error {
	g.gitcmd.MustGit("add -A", nil)
	err := g.gitcmd.Git("commit --amend --no-edit", nil)
	if err != nil {
		return fmt.Errorf("failed to amend commit: %w", err)
	}
	err = g.gitcmd.Git("rebase --continue", nil)
	if err != nil {
		return fmt.Errorf("rebase conflict detected: %w", err)
	}
	os.Remove(g.EditStatePath())
	return nil
}

// EditAbort cancels the current edit session.
// Extracted from spr.go EditCommitAbort().
func (g *GitOps) EditAbort() error {
	err := g.gitcmd.Git("rebase --abort", nil)
	if err != nil {
		return fmt.Errorf("failed to abort rebase: %w", err)
	}
	os.Remove(g.EditStatePath())
	return nil
}

// PrepareForPush stashes uncommitted changes and returns a cleanup function.
// Extracted from spr.go syncCommitStackToGitHub().
func (g *GitOps) PrepareForPush() (func(), error) {
	var output string
	g.gitcmd.MustGit("status --porcelain --untracked-files=no", &output)
	if output != "" {
		err := g.gitcmd.Git("stash", nil)
		if err != nil {
			return nil, err
		}
		return func() { g.gitcmd.MustGit("stash pop", nil) }, nil
	}
	return func() {}, nil
}

// IsEditing returns true if an edit session is in progress.
func (g *GitOps) IsEditing() bool {
	_, err := os.Stat(g.EditStatePath())
	return err == nil
}

// EditStatePath returns the path to the edit state file.
func (g *GitOps) EditStatePath() string {
	return filepath.Join(g.gitcmd.RootDir(), ".git", "spr_edit_state")
}

