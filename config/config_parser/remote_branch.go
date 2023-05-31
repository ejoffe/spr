package config_parser

import (
	"fmt"
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
		fmt.Printf("error: unable to fetch remote branch info, using defaults")
		return
	}

	internalCfg := cfg.(*config.InternalConfig)

	internalCfg.GitHubRemote = matches[2]
	internalCfg.GitHubBranch = matches[3]
}
