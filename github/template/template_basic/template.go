package template_basic

import (
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/template"
)

type BasicTemplatizer struct{}

func NewBasicTemplatizer() *BasicTemplatizer {
	return &BasicTemplatizer{}
}

func (t *BasicTemplatizer) Title(info *github.GitHubInfo, commit git.Commit) string {
	return commit.Subject
}

func (t *BasicTemplatizer) Body(info *github.GitHubInfo, commit git.Commit, pr *github.PullRequest) string {
	body := commit.Body
	body += "\n\n"
	body += template.ManualMergeNotice()
	return body
}
