package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/git"
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

func ConfigFilePath(gitcmd git.Cmd) string {
	var rootdir string
	err := gitcmd("rev-parse --show-toplevel", &rootdir)
	check(err)
	rootdir = strings.TrimSpace(rootdir)
	filepath := filepath.Clean(rootdir + "/.spr.yml")
	return filepath
}

func GitHubRemoteSource(config *Config, gitcmd git.Cmd) *remoteSource {
	return &remoteSource{
		gitcmd: gitcmd,
		config: config,
	}
}

type remoteSource struct {
	gitcmd git.Cmd
	config *Config
}

func (s *remoteSource) Load(_ interface{}) {
	var output string
	err := s.gitcmd("remote -v", &output)
	check(err)
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
