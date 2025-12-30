package template_custom

import (
	"errors"
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
	return body
}

func (t *CustomTemplatizer) formatBody(commit git.Commit, stack []*github.PullRequest) string {

	if len(stack) <= 1 {
		return strings.TrimSpace(commit.Body)
	}

	if commit.Body == "" {
		return fmt.Sprintf(
			"**Stack**:\n%s\n%s",
			template.FormatStackMarkdown(commit, stack, t.repoConfig.ShowPrTitlesInStack),
			template.ManualMergeNotice(),
		)
	}

	return fmt.Sprintf("%s\n\n---\n\n**Stack**:\n%s\n%s",
		commit.Body,
		template.FormatStackMarkdown(commit, stack, t.repoConfig.ShowPrTitlesInStack),
		template.ManualMergeNotice(),
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

const (
	// Default anchors mark where the stack content is inserted in PR templates.
	// Can be overridden in RepoConfig using PRTemplateInsertStart and PRTemplateInsertEnd.
	// If anchors are not found in the template, content is appended to the PR.
	// Implemented as HTML comments so they don't appear in rendered Markdown.

	defaultStartAnchor = "<!-- SPR-STACK-START -->"
	defaultEndAnchor   = "<!-- SPR-STACK-END -->"
)

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

	startAnchor := t.repoConfig.PRTemplateInsertStart
	if startAnchor == "" {
		startAnchor = defaultStartAnchor
	}

	endAnchor := t.repoConfig.PRTemplateInsertEnd
	if endAnchor == "" {
		endAnchor = defaultEndAnchor
	}

	startPRTemplateSection, err := getSectionOfPRTemplate(templateOrExistingPRBody, startAnchor, BeforeMatch)
	if err == ErrNoMatchesFound && startAnchor == defaultStartAnchor {
		// Default append mode: if no anchors found in the template, append body at the end.
		return fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n", templateOrExistingPRBody, startAnchor, body, endAnchor), nil
	}

	if err != nil {
		return "", fmt.Errorf("%w: PR template insert start = '%v'", err, startAnchor)
	}

	endPRTemplateSection, err := getSectionOfPRTemplate(templateOrExistingPRBody, endAnchor, AfterMatch)
	if err != nil {
		return "", fmt.Errorf("%w: PR template insert end = '%v'", err, endAnchor)
	}

	return fmt.Sprintf("%v%v\n%v\n\n%v%v", startPRTemplateSection, startAnchor, body, endAnchor, endPRTemplateSection), nil
}

const (
	BeforeMatch = iota
	AfterMatch
)

var (
	// Error returned when no matches are found in a PR template
	ErrNoMatchesFound = fmt.Errorf("no matches found")
	// Error returned when multiple matches are found in a PR template
	ErrMultipleMatchesFound = fmt.Errorf("multiple matches found")
)

// getSectionOfPRTemplate searches text for a matching searchString and will return the text before or after the
// match as a string. If there are no matches or more than one match is found, an error will be returned.
func getSectionOfPRTemplate(text, searchString string, returnMatch int) (string, error) {
	// Check occurrence count in a single pass
	count := strings.Count(text, searchString)
	switch count {
	case 0:
		return "", ErrNoMatchesFound
	case 1:
		// Expected case: exactly one match
		idx := strings.Index(text, searchString)
		switch returnMatch {
		case BeforeMatch:
			return text[:idx], nil
		case AfterMatch:
			return text[idx+len(searchString):], nil
		default:
			return "", errors.New("invalid enum value")
		}
	default:
		// count > 1
		return "", ErrMultipleMatchesFound
	}
}
