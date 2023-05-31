package config_parser

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

func ParseConfig(gitcmd git.GitInterface) *config.Config {
	cfg := config.EmptyConfig()

	rake.LoadSources(cfg.Repo,
		rake.DefaultSource(),
		NewGitHubRemoteSource(cfg, gitcmd),
		rake.YamlFileSource(RepoConfigFilePath(gitcmd)),
		rake.YamlFileWriter(RepoConfigFilePath(gitcmd)),
	)
	if cfg.Repo.GitHubHost == "" {
		fmt.Println("unable to auto configure repository host - must be set manually in .spr.yml")
		os.Exit(2)
	}
	if cfg.Repo.GitHubRepoOwner == "" {
		fmt.Println("unable to auto configure repository owner - must be set manually in .spr.yml")
		os.Exit(3)
	}

	if cfg.Repo.GitHubRepoName == "" {
		fmt.Println("unable to auto configure repository name - must be set manually in .spr.yml")
		os.Exit(4)
	}

	rake.LoadSources(cfg.User,
		rake.DefaultSource(),
		rake.YamlFileSource(UserConfigFilePath()),
	)

	rake.LoadSources(cfg.Internal,
		rake.DefaultSource(),
		NewRemoteBranchSource(gitcmd),
	)

	rake.LoadSources(cfg.State,
		rake.DefaultSource(),
		rake.YamlFileSource(InternalConfigFilePath()),
	)

	rake.LoadSources(cfg.User,
		rake.YamlFileWriter(UserConfigFilePath()))

	cfg.State.RunCount = cfg.State.RunCount + 1

	rake.LoadSources(cfg.State,
		rake.YamlFileWriter(InternalConfigFilePath()))

	return cfg
}

func RepoConfigFilePath(gitcmd git.GitInterface) string {
	rootdir := gitcmd.RootDir()
	filepath := filepath.Clean(path.Join(rootdir, ".spr.yml"))
	return filepath
}

func UserConfigFilePath() string {
	rootdir, err := os.UserHomeDir()
	check(err)
	filepath := filepath.Clean(path.Join(rootdir, ".spr.yml"))
	return filepath
}

func InternalConfigFilePath() string {
	rootdir, err := os.UserHomeDir()
	check(err)
	filepath := filepath.Clean(path.Join(rootdir, ".spr.state"))
	return filepath
}
