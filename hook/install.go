package hook

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ejoffe/spr/git"
)

const (
	hookPath = ".git/hooks/commit-msg"
)

func InstallCommitHook(gitcmd git.GitInterface) {
	var rootdir string
	err := gitcmd.Git("rev-parse --show-toplevel", &rootdir)
	check(err)
	rootdir = strings.TrimSpace(rootdir)
	err = os.Chdir(rootdir)
	check(err)

	info, err := os.Lstat(hookPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			linkPath, err := os.Readlink(hookPath)
			check(err)
			if !strings.HasSuffix(linkPath, "spr_commit_hook") {
				panic("different commit hook already installed")
			}
		}
		// amend commit stack to add commit-id
		rewordPath, err := exec.LookPath("spr_reword_helper")
		check(err)
		gitcmd.GitWithEditor("rebase origin/master -i --autosquash --autostash", nil, rewordPath)
	} else {
		binPath, err := exec.LookPath("spr_commit_hook")
		check(err)
		err = os.Symlink(binPath, hookPath)
		check(err)
		fmt.Printf("Installed commit hook in .git/hooks/commit-msg\n")
		// amend commit stack to add commit-id
		rewordPath, err := exec.LookPath("spr_reword_helper")
		check(err)
		gitcmd.GitWithEditor("rebase origin/master -i --autosquash --autostash", nil, rewordPath)
	}
}
