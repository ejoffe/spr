package config_parser

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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
		NewRemoteBranchSource(gitcmd),
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

	rake.LoadSources(cfg.State,
		rake.DefaultSource(),
		rake.YamlFileSource(InternalConfigFilePath()),
	)

	cfg.State.RunCount = cfg.State.RunCount + 1

	rake.LoadSources(cfg.State,
		rake.YamlFileWriter(InternalConfigFilePath()))

	// init case : if yaml config files not found : create them
	if _, err := os.Stat(RepoConfigFilePath(gitcmd)); errors.Is(err, os.ErrNotExist) {
		rake.LoadSources(cfg.Repo,
			rake.YamlFileWriter(RepoConfigFilePath(gitcmd)))
	}

	if _, err := os.Stat(UserConfigFilePath()); errors.Is(err, os.ErrNotExist) {
		rake.LoadSources(cfg.User,
			rake.YamlFileWriter(UserConfigFilePath()))
	}
	return cfg
}

func CheckConfig(cfg *config.Config) error {
	if strings.Contains(cfg.Repo.GitHubBranch, "/") {
		return errors.New("Remote branch name must not contain backslashes '/'")
	}
	return nil
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
