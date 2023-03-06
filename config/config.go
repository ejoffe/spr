package config

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
)

type Config struct {
	Repo *RepoConfig
	User *UserConfig
}

// Config object to hold spr configuration
type RepoConfig struct {
	GitHubRepoOwner string `yaml:"githubRepoOwner"`
	GitHubRepoName  string `yaml:"githubRepoName"`
	GitHubHost      string `default:"github.com" yaml:"githubHost"`

	RequireChecks   bool `default:"true" yaml:"requireChecks"`
	RequireApproval bool `default:"true" yaml:"requireApproval"`

	GitHubRemote   string   `default:"origin" yaml:"githubRemote"`
	GitHubBranch   string   `default:"master" yaml:"githubBranch"`
	RemoteBranches []string `yaml:"remoteBranches"`

	MergeMethod string `default:"rebase" yaml:"mergeMethod"`

	PRTemplatePath        string `yaml:"prTemplatePath,omitempty"`
	PRTemplateInsertStart string `yaml:"prTemplateInsertStart,omitempty"`
	PRTemplateInsertEnd   string `yaml:"prTemplateInsertEnd,omitempty"`
}

type UserConfig struct {
	ShowPRLink       bool `default:"true" yaml:"showPRLink"`
	LogGitCommands   bool `default:"true" yaml:"logGitCommands"`
	LogGitHubCalls   bool `default:"true" yaml:"logGitHubCalls"`
	StatusBitsHeader bool `default:"true" yaml:"statusBitsHeader"`
	StatusBitsEmojis bool `default:"true" yaml:"statusBitsEmojis"`

	CreateDraftPRs       bool `default:"false" yaml:"createDraftPRs"`
	PreserveTitleAndBody bool `default:"false" yaml:"preserveTitleAndBody"`
	NoRebase             bool `default:"false" yaml:"noRebase"`

	Stargazer bool `default:"false" yaml:"stargazer"`
	RunCount  int  `default:"0" yaml:"runcount"`
}

func EmptyConfig() *Config {
	return &Config{
		Repo: &RepoConfig{},
		User: &UserConfig{},
	}
}

func DefaultConfig() *Config {
	cfg := EmptyConfig()
	rake.LoadSources(cfg.Repo,
		rake.DefaultSource(),
	)
	rake.LoadSources(cfg.User,
		rake.DefaultSource(),
	)

	cfg.User.LogGitCommands = false
	cfg.User.LogGitHubCalls = false
	return cfg
}

func ParseConfig(gitcmd git.GitInterface) *Config {
	cfg := EmptyConfig()

	rake.LoadSources(cfg.Repo,
		rake.DefaultSource(),
		GitHubRemoteSource(cfg, gitcmd),
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

	if !cfg.User.Stargazer {
		cfg.User.RunCount = cfg.User.RunCount + 1
	}

	rake.LoadSources(cfg.User,
		rake.YamlFileWriter(UserConfigFilePath()))

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

func GitHubRemoteSource(config *Config, gitcmd git.GitInterface) *remoteSource {
	return &remoteSource{
		gitcmd: gitcmd,
		config: config,
	}
}

func (c Config) MergeMethod() (genclient.PullRequestMergeMethod, error) {
	var mergeMethod genclient.PullRequestMergeMethod
	var err error
	switch strings.ToLower(c.Repo.MergeMethod) {
	case "merge":
		mergeMethod = genclient.PullRequestMergeMethod_MERGE
	case "squash":
		mergeMethod = genclient.PullRequestMergeMethod_SQUASH
	case "rebase", "":
		mergeMethod = genclient.PullRequestMergeMethod_REBASE
	default:
		err = fmt.Errorf(
			`unknown merge method %q, choose from "merge", "squash", or "rebase"`,
			c.Repo.MergeMethod,
		)
	}
	return mergeMethod, err
}

type remoteSource struct {
	gitcmd git.GitInterface
	config *Config
}

func (s *remoteSource) Load(_ interface{}) {
	var output string
	err := s.gitcmd.Git("remote -v", &output)
	check(err)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		githubHost, repoOwner, repoName, match := getRepoDetailsFromRemote(line)
		if match {
			s.config.Repo.GitHubHost = githubHost
			s.config.Repo.GitHubRepoOwner = repoOwner
			s.config.Repo.GitHubRepoName = repoName
			break
		}
	}
}

func getRepoDetailsFromRemote(remote string) (string, string, string, bool) {
	// Allows "https://", "ssh://" or no protocol at all (this means ssh)
	protocolFormat := `(?:(https://)|(ssh://))?`
	// This may or may not be present in the address
	userFormat := `(git@)?`
	// "/" is expected in "http://" or "ssh://" protocol, when no protocol given
	// it should be ":"
	repoFormat := `(?P<githubHost>[a-z0-9._\-]+)(/|:)(?P<repoOwner>\w+)/(?P<repoName>[\w-]+)`
	// This is neither required in https access nor in ssh one
	suffixFormat := `(.git)?`
	regexFormat := fmt.Sprintf(`^origin\s+%s%s%s%s \(push\)`,
		protocolFormat, userFormat, repoFormat, suffixFormat)
	regex := regexp.MustCompile(regexFormat)
	matches := regex.FindStringSubmatch(remote)
	if matches != nil {
		githubHostIndex := regex.SubexpIndex("githubHost")
		repoOwnerIndex := regex.SubexpIndex("repoOwner")
		repoNameIndex := regex.SubexpIndex("repoName")
		return matches[githubHostIndex], matches[repoOwnerIndex], matches[repoNameIndex], true
	}
	return "", "", "", false
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

/*
func installCommitHook() {
	var rootdir string
	mustgit("rev-parse --show-toplevel", &rootdir)
	rootdir = strings.TrimSpace(rootdir)
	err := os.Chdir(rootdir)
	check(err)
	path, err := exec.LookPath("spr_commit_hook")
	check(err)
	cmd := exec.Command("ln", "-s", path, ".git/hooks/commit-msg")
	_, err = cmd.CombinedOutput()
	check(err)
	fmt.Printf("- Installed commit hook in .git/hooks/commit-msg\n")
}
*/
