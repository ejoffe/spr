package template_stack

import (
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/template"
)

type StackTemplatizer struct {
	showPrTitlesInStack bool
}

func NewStackTemplatizer(showPrTitlesInStack bool) *StackTemplatizer {
	return &StackTemplatizer{showPrTitlesInStack: showPrTitlesInStack}
}

func (t *StackTemplatizer) Title(info *github.GitHubInfo, commit git.Commit) string {
	return commit.Subject
}

func (t *StackTemplatizer) Body(info *github.GitHubInfo, commit git.Commit, pr *github.PullRequest) string {
	body := commit.Body

	// Always show stack section and notice
	body += "\n"
	body += "---\n"
	body += "**Stack**:\n"
	body += template.FormatStackMarkdown(commit, info.PullRequests, t.showPrTitlesInStack)
	body += "---\n"
	body += template.ManualMergeNotice()
	return body
}
