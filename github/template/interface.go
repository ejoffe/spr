package template

import (
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
)

type PRTemplatizer interface {
	Title(info *github.GitHubInfo, commit git.Commit) string
	// Body renders the PR body. stack is the ordered list of PRs in the
	// stack (bottom to top) used to render the stack section, independent
	// from info.PullRequests which may carry an unrelated order.
	Body(info *github.GitHubInfo, stack []*github.PullRequest, commit git.Commit, pr *github.PullRequest) string
}
