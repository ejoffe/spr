package config_parser

import (
	"regexp"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

type remoteBranch struct {
	gitcmd git.GitInterface
}

func NewRemoteBranchSource(gitcmd git.GitInterface) *remoteBranch {
	return &remoteBranch{
		gitcmd: gitcmd,
	}
}

var _remoteBranchRegex = regexp.MustCompile(`^## ([a-zA-Z0-9_\-/\.]+)\.\.\.([a-zA-Z0-9_\-/\.]+)/([a-zA-Z0-9_\-/\.]+)`)

func (s *remoteBranch) Load(cfg interface{}) {
	var output string
	err := s.gitcmd.Git("status -b --porcelain -u no", &output)
	check(err)

	matches := _remoteBranchRegex.FindStringSubmatch(output)
	if matches == nil {
		return
	}

	repoCfg := cfg.(*config.RepoConfig)

	repoCfg.GitHubRemote = matches[2]
	repoCfg.GitHubBranch = matches[3]
}
