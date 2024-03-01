package config_parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
)

type remoteSource struct {
	gitcmd git.GitInterface
	config *config.Config
}

func NewGitHubRemoteSource(config *config.Config, gitcmd git.GitInterface) *remoteSource {
	return &remoteSource{
		gitcmd: gitcmd,
		config: config,
	}
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
	repoFormat := `(?P<githubHost>[a-z0-9._\-]+)(/|:)(?P<repoOwner>[\w-]+)/(?P<repoName>[\w-]+)`
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
