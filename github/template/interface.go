package template

import (
	"github.com/ejoffe/spr/forge"
	"github.com/ejoffe/spr/git"
)

type PRTemplatizer interface {
	Title(info *forge.ForgeInfo, commit git.Commit) string
	Body(info *forge.ForgeInfo, commit git.Commit, pr *forge.PullRequest) string
}
