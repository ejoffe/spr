package config_parser

import (
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

// originPushURLRegex captures the URL portion of an `origin ... (push)` line
// from `git remote -v` output. The URL itself is parsed by git.ParseRepoURL.
var originPushURLRegex = regexp.MustCompile(`^origin\s+(\S+)\s+\(push\)`)

func getRepoDetailsFromRemote(remote string) (string, string, string, bool) {
	matches := originPushURLRegex.FindStringSubmatch(remote)
	if matches == nil {
		return "", "", "", false
	}
	return git.ParseRepoURL(matches[1])
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
