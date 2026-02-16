package template_basic

import (
	"github.com/ejoffe/spr/forge"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/template"
)

type BasicTemplatizer struct{}

func NewBasicTemplatizer() *BasicTemplatizer {
	return &BasicTemplatizer{}
}

func (t *BasicTemplatizer) Title(info *forge.ForgeInfo, commit git.Commit) string {
	return commit.Subject
}

func (t *BasicTemplatizer) Body(info *forge.ForgeInfo, commit git.Commit, pr *forge.PullRequest) string {
	body := commit.Body
	body += "\n\n"
	body += template.ManualMergeNotice()
	return body
}
