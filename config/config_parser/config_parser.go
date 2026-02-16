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
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// migrateRepoConfigKeys migrates old GitHub-specific YAML keys to
// forge-agnostic names in the given .spr.yml file. If the file contains
// any legacy keys they are renamed in place and the file is rewritten.
func migrateRepoConfigKeys(cfgPath string) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return // file doesn't exist yet or is unreadable; nothing to migrate
	}

	var raw yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}

	renames := map[string]string{
		"githubRepoOwner": "repoOwner",
		"githubRepoName":  "repoName",
		"githubHost":      "forgeHost",
		"githubRemote":    "remote",
		"githubBranch":    "branch",
	}

	// The top-level node is a document; its first child is the mapping.
	if raw.Kind != yaml.DocumentNode || len(raw.Content) == 0 {
		return
	}
	mapping := raw.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return
	}

	migrated := false
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		if newKey, ok := renames[keyNode.Value]; ok {
			keyNode.Value = newKey
			migrated = true
		}
	}

	if !migrated {
		return
	}

	out, err := yaml.Marshal(&raw)
	if err != nil {
		log.Warn().Err(err).Msg("failed to marshal migrated config")
		return
	}
	if err := os.WriteFile(cfgPath, out, 0644); err != nil {
		log.Warn().Err(err).Msg("failed to write migrated config")
	}
}

func ParseConfig(gitcmd git.GitInterface) *config.Config {
	cfg := config.EmptyConfig()

	// Migrate legacy GitHub-specific config keys before loading.
	migrateRepoConfigKeys(RepoConfigFilePath(gitcmd))

	rake.LoadSources(cfg.Repo,
		rake.DefaultSource(),
		NewRemoteSource(cfg, gitcmd),
		rake.YamlFileSource(RepoConfigFilePath(gitcmd)),
		NewRemoteBranchSource(gitcmd),
	)
	if cfg.Repo.ForgeHost == "" {
		fmt.Println("unable to auto configure repository host - must be set manually in .spr.yml")
		os.Exit(2)
	}
	if cfg.Repo.RepoOwner == "" {
		fmt.Println("unable to auto configure repository owner - must be set manually in .spr.yml")
		os.Exit(3)
	}

	if cfg.Repo.RepoName == "" {
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

	// Normalize config (e.g., set PRTemplateType to "custom" if PRTemplatePath is provided)
	cfg.Normalize()

	return cfg
}

func CheckConfig(cfg *config.Config) error {
	if strings.Contains(cfg.Repo.Branch, "/") {
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
