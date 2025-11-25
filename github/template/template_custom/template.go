package template_custom

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/template"
	"github.com/rs/zerolog/log"
)

type CustomTemplatizer struct {
	repoConfig *config.RepoConfig
	gitcmd     git.GitInterface
}

func NewCustomTemplatizer(
	repoConfig *config.RepoConfig,
	gitcmd git.GitInterface,
) *CustomTemplatizer {
	return &CustomTemplatizer{
		repoConfig: repoConfig,
		gitcmd:     gitcmd,
	}
}

func (t *CustomTemplatizer) Title(info *github.GitHubInfo, commit git.Commit) string {
	return commit.Subject
}

func (t *CustomTemplatizer) Body(info *github.GitHubInfo, commit git.Commit) string {
	body := t.formatBody(commit, info.PullRequests)
	pullRequestTemplate, err := t.readPRTemplate()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read PR template")
	}
	body, err = t.insertBodyIntoPRTemplate(body, pullRequestTemplate, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to insert body into PR template")
	}
	return commit.Body
}

func (t *CustomTemplatizer) formatBody(commit git.Commit, stack []*github.PullRequest) string {

	if len(stack) <= 1 {
		return strings.TrimSpace(commit.Body)
	}

	if commit.Body == "" {
		return fmt.Sprintf(
			"**Stack**:\n%s",
			template.AddManualMergeNotice(
				template.FormatStackMarkdown(commit, stack, t.repoConfig.ShowPrTitlesInStack),
			),
		)
	}

	return fmt.Sprintf("%s\n\n---\n\n**Stack**:\n%s",
		commit.Body,
		template.AddManualMergeNotice(
			template.FormatStackMarkdown(commit, stack, t.repoConfig.ShowPrTitlesInStack),
		),
	)
}

// Reads the specified PR template file and returns it as a string
func (t *CustomTemplatizer) readPRTemplate() (string, error) {
	repoRootDir := t.gitcmd.RootDir()
	fullTemplatePath := filepath.Clean(path.Join(repoRootDir, t.repoConfig.PRTemplatePath))
	pullRequestTemplateBytes, err := os.ReadFile(fullTemplatePath)
	if err != nil {
		return "", fmt.Errorf("%w: unable to read template %v", err, fullTemplatePath)
	}
	return string(pullRequestTemplateBytes), nil
}

// insertBodyIntoPRTemplate inserts a text body into the given PR template and returns the result as a string.
// It uses the PRTemplateInsertStart and PRTemplateInsertEnd values defined in RepoConfig to determine where the body
// should be inserted in the PR template. If there are issues finding the correct place to insert the body
// an error will be returned.
//
// NOTE: on PR update, rather than using the PR template, it will use the existing PR body, which should have
// the PR template from the initial PR create.
func (t *CustomTemplatizer) insertBodyIntoPRTemplate(body, prTemplate string, pr *github.PullRequest) (string, error) {
	templateOrExistingPRBody := prTemplate
	if pr != nil && pr.Body != "" {
		templateOrExistingPRBody = pr.Body
	}

	startPRTemplateSection, err := getSectionOfPRTemplate(templateOrExistingPRBody, t.repoConfig.PRTemplateInsertStart, BeforeMatch)
	if err != nil {
		return "", fmt.Errorf("%w: PR template insert start = '%v'", err, t.repoConfig.PRTemplateInsertStart)
	}

	endPRTemplateSection, err := getSectionOfPRTemplate(templateOrExistingPRBody, t.repoConfig.PRTemplateInsertEnd, AfterMatch)
	if err != nil {
		return "", fmt.Errorf("%w: PR template insert end = '%v'", err, t.repoConfig.PRTemplateInsertStart)
	}

	return fmt.Sprintf("%v%v\n%v\n\n%v%v", startPRTemplateSection, t.repoConfig.PRTemplateInsertStart, body,
		t.repoConfig.PRTemplateInsertEnd, endPRTemplateSection), nil
}

const (
	BeforeMatch = iota
	AfterMatch
)

// getSectionOfPRTemplate searches text for a matching searchString and will return the text before or after the
// match as a string. If there are no matches or more than one match is found, an error will be returned.
func getSectionOfPRTemplate(text, searchString string, returnMatch int) (string, error) {
	split := strings.Split(text, searchString)
	switch len(split) {
	case 2:
		if returnMatch == BeforeMatch {
			return split[0], nil
		} else if returnMatch == AfterMatch {
			return split[1], nil
		}
		return "", fmt.Errorf("invalid enum value")
	case 1:
		return "", fmt.Errorf("no matches found")
	default:
		return "", fmt.Errorf("multiple matches found")
	}
}
