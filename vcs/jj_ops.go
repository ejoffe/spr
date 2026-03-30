package vcs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/google/uuid"
)

// JjOps implements VCSOperations using jj (Jujutsu) commands.
// Git commands are still used for push operations (via gitcmd).
type JjOps struct {
	cfg    *config.Config
	jjcmd  JjInterface
	gitcmd git.GitInterface
}

// NewJjOps creates a jj-based VCSOperations implementation.
func NewJjOps(cfg *config.Config, jjcmd JjInterface, gitcmd git.GitInterface) *JjOps {
	return &JjOps{cfg: cfg, jjcmd: jjcmd, gitcmd: gitcmd}
}

// FetchAndRebase fetches from remote and rebases using jj commands.
// Preserves jj change IDs (unlike git rebase which destroys them).
func (j *JjOps) FetchAndRebase(cfg *config.Config) error {
	if cfg.User.NoRebase {
		// Only fetch, skip rebase (same semantics as git NoRebase)
		return j.jjcmd.Jj("git fetch", nil)
	}

	err := j.jjcmd.Jj("git fetch", nil)
	if err != nil {
		return err
	}

	// Rebase current stack onto updated trunk
	remote := cfg.Repo.GitHubRemote
	branch := cfg.Repo.GitHubBranch
	rebaseCmd := fmt.Sprintf("rebase -b @ -d %s@%s", branch, remote)
	return j.jjcmd.Jj(rebaseCmd, nil)
}

// GetLocalCommitStack returns unmerged commits using jj log.
// If any commits lack commit-id trailers, adds them via jj describe.
func (j *JjOps) GetLocalCommitStack(cfg *config.Config, gitcmd git.GitInterface) []git.Commit {
	template := `commit_id ++ "\x1f" ++ change_id ++ "\x1f" ++ empty ++ "\x1f" ++ description ++ "\x1e"`

	var output string
	err := j.jjcmd.JjArgs([]string{"log", "--no-graph", "--reversed", "--color=never", "-r", "trunk()..@", "-T", template}, &output)
	if err != nil {
		panic(err)
	}

	parsed, valid := parseJjLogOutput(output)

	if !valid {
		// Add commit-id trailers to commits that lack them
		for i, p := range parsed {
			if p.sprCommitID == "" && !p.empty {
				newID := uuid.New().String()[:8]
				newDesc := strings.TrimRight(p.description, "\n")
				newDesc += "\n\ncommit-id:" + newID
				err := j.jjcmd.JjArgs([]string{"describe", "-r", p.changeID, "-m", newDesc}, nil)
				if err != nil {
					panic(fmt.Sprintf("failed to add commit-id to %s: %v", p.changeID, err))
				}
				parsed[i].sprCommitID = newID
			}
		}

		// Re-read commit hashes since jj describe changes them
		err = j.jjcmd.JjArgs([]string{"log", "--no-graph", "--reversed", "--color=never", "-r", "trunk()..@", "-T", template}, &output)
		if err != nil {
			panic(err)
		}
		reparsed, revalid := parseJjLogOutput(output)
		if !revalid {
			panic("unable to add commit-id trailers via jj describe")
		}
		parsed = reparsed
	}

	// Convert to []git.Commit
	var commits []git.Commit
	for _, p := range parsed {
		if p.wip {
			// Include WIP commits but mark them (spr stops at first WIP)
		}
		commits = append(commits, git.Commit{
			CommitID:   p.sprCommitID,
			CommitHash: p.commitHash,
			ChangeID:   p.changeID,
			Subject:    p.subject,
			Body:       p.body,
			WIP:        p.wip,
		})
	}
	return commits
}

// AmendInto squashes working copy changes into a specific commit.
// Uses jj squash which preserves change IDs.
func (j *JjOps) AmendInto(commit git.Commit) error {
	if commit.ChangeID == "" {
		return fmt.Errorf("cannot amend: commit %s has no jj change ID", commit.CommitID)
	}
	return j.jjcmd.Jj(fmt.Sprintf("squash --into %s", commit.ChangeID), nil)
}

// EditStart begins an edit session by checking out the target commit.
// Uses jj edit which preserves change IDs.
func (j *JjOps) EditStart(commit git.Commit) error {
	if commit.ChangeID == "" {
		return fmt.Errorf("cannot edit: commit %s has no jj change ID", commit.CommitID)
	}

	// Save operation ID for abort
	var opID string
	j.jjcmd.MustJj("op log --no-graph -n 1 -T 'id.short(16)'", &opID)

	// Save current @ for finish
	var currentAt string
	j.jjcmd.MustJj("log --no-graph -r @ -T change_id", &currentAt)

	// Write state file
	stateContent := fmt.Sprintf("vcs=jj\nchange_id=%s\noriginal_at=%s\nop_id=%s\ncommit_id=%s\ncommit_subject=%s\n",
		commit.ChangeID, strings.TrimSpace(currentAt), strings.TrimSpace(opID),
		commit.CommitID, commit.Subject)
	err := os.WriteFile(j.EditStatePath(), []byte(stateContent), 0644)
	if err != nil {
		return err
	}

	// Move working copy to the target commit
	err = j.jjcmd.Jj("edit "+commit.ChangeID, nil)
	if err != nil {
		os.Remove(j.EditStatePath())
		return err
	}
	return nil
}

// EditFinish completes an edit session.
// In jj, changes to the edited commit are automatically captured.
// We just need to return to where we were.
func (j *JjOps) EditFinish() error {
	state, err := j.readEditState()
	if err != nil {
		return err
	}

	// Return to where we were before the edit
	err = j.jjcmd.Jj("new "+state["original_at"], nil)
	if err != nil {
		return fmt.Errorf("failed to return from edit: %w", err)
	}

	os.Remove(j.EditStatePath())
	return nil
}

// EditAbort cancels an edit session by restoring the operation state.
func (j *JjOps) EditAbort() error {
	state, err := j.readEditState()
	if err != nil {
		return err
	}

	err = j.jjcmd.Jj("op restore "+state["op_id"], nil)
	if err != nil {
		return fmt.Errorf("failed to restore operation: %w", err)
	}

	os.Remove(j.EditStatePath())
	return nil
}

// PrepareForPush is a no-op for jj — the working copy is always a commit.
func (j *JjOps) PrepareForPush() (func(), error) {
	return func() {}, nil
}

// IsEditing returns true if an edit session is in progress.
func (j *JjOps) IsEditing() bool {
	_, err := os.Stat(j.EditStatePath())
	return err == nil
}

// EditStatePath returns the path to the edit state file.
func (j *JjOps) EditStatePath() string {
	if j.gitcmd != nil {
		return filepath.Join(j.gitcmd.RootDir(), ".git", "spr_edit_state")
	}
	return ""
}

// CheckStackCompleteness warns if @ is not at the top of the stack.
// In jj, this happens when the user has done 'jj edit' to a mid-stack commit,
// causing 'trunk()..@' to miss commits above @.
func (j *JjOps) CheckStackCompleteness() string {
	var output string
	err := j.jjcmd.JjArgs([]string{"log", "--no-graph", "--color=never", "-r", "children(@) & trunk()..@+", "-T", `change_id ++ "\n"`}, &output)
	if err != nil {
		return ""
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	lines := strings.Split(output, "\n")
	return fmt.Sprintf("warning: @ is not at the top of your stack — %d commit(s) above @ will be excluded from spr operations", len(lines))
}

// readEditState reads the key=value state file.
func (j *JjOps) readEditState() (map[string]string, error) {
	data, err := os.ReadFile(j.EditStatePath())
	if err != nil {
		return nil, fmt.Errorf("no edit session in progress: %w", err)
	}
	state := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			state[parts[0]] = parts[1]
		}
	}
	return state, nil
}
