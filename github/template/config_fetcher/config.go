package config_fetcher

import (
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github/template"
	"github.com/ejoffe/spr/github/template/template_basic"
	"github.com/ejoffe/spr/github/template/template_custom"
	"github.com/ejoffe/spr/github/template/template_stack"
	"github.com/ejoffe/spr/github/template/template_why_what"
)

func PRTemplatizer(c *config.Config, gitcmd git.GitInterface) template.PRTemplatizer {
	switch c.Repo.PRTemplateType {
	case "stack":
		return template_stack.NewStackTemplatizer(c.Repo.ShowPrTitlesInStack)
	case "basic":
		return template_basic.NewBasicTemplatizer()
	case "why_what":
		return template_why_what.NewWhyWhatTemplatizer()
	case "custom":
		return template_custom.NewCustomTemplatizer(c.Repo, gitcmd)
	default:
		return template_basic.NewBasicTemplatizer()
	}
}
