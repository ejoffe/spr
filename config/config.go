package config

import (
	"fmt"
	"strings"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/github/githubclient/gen/genclient"
)

type Config struct {
	Repo  *RepoConfig
	User  *UserConfig
	State *InternalState
}

// Config object to hold spr configuration
type RepoConfig struct {
	GitHubRepoOwner string `yaml:"githubRepoOwner"`
	GitHubRepoName  string `yaml:"githubRepoName"`
	GitHubHost      string `default:"github.com" yaml:"githubHost"`

	GitHubRemote string `default:"origin" yaml:"githubRemote"`
	GitHubBranch string `default:"main" yaml:"githubBranch"`

	RequireChecks   bool `default:"true" yaml:"requireChecks"`
	RequireApproval bool `default:"true" yaml:"requireApproval"`

	MergeMethod string `default:"rebase" yaml:"mergeMethod"`
	MergeQueue  bool   `default:"false" yaml:"mergeQueue"`

	PRTemplatePath        string `yaml:"prTemplatePath,omitempty"`
	PRTemplateInsertStart string `yaml:"prTemplateInsertStart,omitempty"`
	PRTemplateInsertEnd   string `yaml:"prTemplateInsertEnd,omitempty"`

	MergeCheck string `yaml:"mergeCheck,omitempty"`

	ForceFetchTags bool `default:"false" yaml:"forceFetchTags"`

	ShowPrTitlesInStack    bool `default:"false" yaml:"showPrTitlesInStack"`
	BranchPushIndividually bool `default:"false" yaml:"branchPushIndividually"`
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
	DeleteMergedBranches bool `default:"false" yaml:"deleteMergedBranches"`
}

type InternalState struct {
	MergeCheckCommit map[string]string `yaml:"mergeCheckCommit"`

	Stargazer bool `default:"false" yaml:"stargazer"`
	RunCount  int  `default:"0" yaml:"runcount"`
}

func EmptyConfig() *Config {
	return &Config{
		Repo: &RepoConfig{},
		User: &UserConfig{},
		State: &InternalState{
			MergeCheckCommit: map[string]string{},
		},
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
