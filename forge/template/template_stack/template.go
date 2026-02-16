package template_stack

import (
	"github.com/ejoffe/spr/forge"
	"github.com/ejoffe/spr/forge/template"
	"github.com/ejoffe/spr/git"
)

type StackTemplatizer struct {
	showPrTitlesInStack bool
}

func NewStackTemplatizer(showPrTitlesInStack bool) *StackTemplatizer {
	return &StackTemplatizer{showPrTitlesInStack: showPrTitlesInStack}
}

func (t *StackTemplatizer) Title(info *forge.ForgeInfo, commit git.Commit) string {
	return commit.Subject
}

func (t *StackTemplatizer) Body(info *forge.ForgeInfo, commit git.Commit, pr *forge.PullRequest) string {
	body := commit.Body

	// Always show stack section and notice
	body += "\n\n"
	body += "---\n"
	body += "**Stack**:\n"
	body += template.FormatStackMarkdown(commit, info.PullRequests, t.showPrTitlesInStack, info.PRNumberPrefix)
	body += "---\n"
	body += template.ManualMergeNotice()
	return body
}
