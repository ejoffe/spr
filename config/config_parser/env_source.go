package config_parser

import (
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"os"
)

type envSource struct {
	gitcmd git.GitInterface
}

func NewEnvSource() *envSource {
	return &envSource{}
}

func (s *envSource) Load(cfg interface{}) {
	repoCfg := cfg.(*config.RepoConfig)

	gitHubBranch := os.Getenv("SPR_GITHUB_BRANCH")
	if gitHubBranch != "" {
		repoCfg.GitHubBranch = gitHubBranch
	}
}
