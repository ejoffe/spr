package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Config object to hold spr configuration
type Config struct {
	GitHubRepoOwner string `yaml:"githubRepoOwner"`
	GitHubRepoName  string `yaml:"githubRepoName"`

	RequireChecks       bool `default:"true" yaml:"requireChecks"`
	RequireApproval     bool `default:"true" yaml:"requireApproval"`
	ShowPRLink          bool `default:"true" yaml:"showPRLink"`
	CleanupRemoteBranch bool `default:"true" yaml:"cleanupRemoteBranch"`
}

func ConfigFilePath() string {
	var rootdir string
	err := git("rev-parse --show-toplevel", &rootdir)
	check(err)
	rootdir = strings.TrimSpace(rootdir)
	filepath := filepath.Clean(rootdir + "/.spr.yml")
	return filepath
}

func GitHubRemoteSource(config *Config) *remoteSource {
	return &remoteSource{
		config: config,
	}
}

type remoteSource struct {
	config *Config
}

func (s *remoteSource) Load(_ interface{}) {
	var output string
	mustgit("remote -v", &output)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		repoOwner, repoName, match := getRepoDetailsFromRemote(line)
		if match {
			s.config.GitHubRepoOwner = repoOwner
			s.config.GitHubRepoName = repoName
			break
		}
	}
}

func getRepoDetailsFromRemote(remote string) (string, string, bool) {
	// Allows "https://", "ssh://" or no protocol at all (this means ssh)
	protocolFormat := `(?:(https://)|(ssh://))?`
	// This may or may not be present in the address
	userFormat := `(git@)?`
	// "/" is expected in "http://" or "ssh://" protocol, when no protocol given
	// it should be ":"
	repoFormat := `github.com(/|:)(?P<repoOwner>\w+)/(?P<repoName>\w+)`
	// This is neither required in https access nor in ssh one
	suffixFormat := `(.git)?`
	regexFormat := fmt.Sprintf(`^origin\s+%s%s%s%s \(push\)`,
		protocolFormat, userFormat, repoFormat, suffixFormat)
	regex := regexp.MustCompile(regexFormat)
	matches := regex.FindStringSubmatch(remote)
	if matches != nil {
		repoOwnerIndex := regex.SubexpIndex("repoOwner")
		repoNameIndex := regex.SubexpIndex("repoName")
		return matches[repoOwnerIndex], matches[repoNameIndex], true
	}
	return "", "", false
}

func mustgit(argStr string, output *string) {
	err := git(argStr, output)
	check(err)
}

func git(argStr string, output *string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command
	args := strings.Split(argStr, " ")
	cmd := exec.Command("git", args...)
	envVarsToDerive := []string{
		"SSH_AUTH_SOCK",
		"SSH_AGENT_PID",
		"HOME",
		"XDG_CONFIG_HOME",
	}
	cmd.Env = []string{"EDITOR=/usr/bin/true"}
	for _, env := range envVarsToDerive {
		envval := os.Getenv(env)
		if envval != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env, envval))
		}
	}

	if output != nil {
		out, err := cmd.CombinedOutput()
		*output = strings.TrimSpace(string(out))
		if err != nil {
			return err
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "git error: %s", string(out))
			return err
		}
	}
	return nil
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
