package template

import (
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

type PRTemplatizer interface {
	Title(info *github.GitHubInfo, commit git.Commit) string
	Body(info *github.GitHubInfo, commit git.Commit, pr *github.PullRequest) string
}
