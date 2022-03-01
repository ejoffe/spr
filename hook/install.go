package hook

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

const (
	hookPath = "hooks/commit-msg"
)

func InstallCommitHook(cfg *config.Config, gitcmd git.GitInterface) {
	var rootdir string
	err := gitcmd.Git("rev-parse --git-common-dir", &rootdir)
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
		rebaseCommand := fmt.Sprintf("rebase %s/%s -i --autosquash --autostash",
			cfg.Repo.GitHubRemote, cfg.Repo.GitHubBranch)
		gitcmd.GitWithEditor(rebaseCommand, nil, rewordPath)
	} else {
		binPath, err := exec.LookPath("spr_commit_hook")
		check(err)
		err = os.Symlink(binPath, hookPath)
		check(err)
		fmt.Printf("Installed commit hook in .git/hooks/commit-msg\n")
		// amend commit stack to add commit-id
		rewordPath, err := exec.LookPath("spr_reword_helper")
		check(err)
		rebaseCommand := fmt.Sprintf("rebase %s/%s -i --autosquash --autostash",
			cfg.Repo.GitHubRemote, cfg.Repo.GitHubBranch)
		gitcmd.GitWithEditor(rebaseCommand, nil, rewordPath)
	}
}
