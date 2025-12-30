package template_why_what

import (
	"bytes"
	"strings"
	go_template "text/template"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/ejoffe/spr/github/template"
)

type WhyWhatTemplatizer struct{}

func NewWhyWhatTemplatizer() *WhyWhatTemplatizer {
	return &WhyWhatTemplatizer{}
}

func (t *WhyWhatTemplatizer) Title(info *github.GitHubInfo, commit git.Commit) string {
	return commit.Subject
}

func (t *WhyWhatTemplatizer) Body(info *github.GitHubInfo, commit git.Commit, pr *github.PullRequest) string {
	// Split commit body by empty lines and filter out empty sections
	sections := splitByEmptyLines(commit.Body)

	// Extract sections: first = Why, second = WhatChanged, third = TestPlan
	// Multiple newlines between sections are treated the same as single newline
	var why, whatChanged, testPlan string
	if len(sections) > 0 {
		why = sections[0]
	}
	if len(sections) > 1 {
		whatChanged = sections[1]
	}
	if len(sections) > 2 {
		testPlan = sections[2]
	}

	// Prepare template data
	data := struct {
		Why         string
		WhatChanged string
		TestPlan    string
	}{
		Why:         why,
		WhatChanged: whatChanged,
		TestPlan:    testPlan,
	}

	// Parse and execute template
	tmpl, err := go_template.New("why_what").Parse(whyWhatTemplate)
	if err != nil {
		// If template parsing fails, return the original body
		return commit.Body
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// If template execution fails, return the original body
		return commit.Body
	}

	body := buf.String()

	// Always show stack section and notice
	body += "\n"
	body += "---\n"
	body += "**Stack**:\n"
	body += template.FormatStackMarkdown(commit, info.PullRequests, true)
	body += "---\n"
	body += template.ManualMergeNotice()
	return body
}

// splitByEmptyLines splits a string by empty lines (one or more consecutive newlines)
// Multiple consecutive newlines are treated as a single separator
// Empty sections are filtered out - only non-empty sections are returned
func splitByEmptyLines(text string) []string {
	if text == "" {
		return []string{}
	}

	// Split by double newline (handles multiple newlines as single separator)
	parts := strings.Split(text, "\n\n")
	sections := make([]string, 0)

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		// Only include non-empty sections
		if trimmed != "" {
			sections = append(sections, trimmed)
		}
	}

	// If no sections found but text exists (single newline case), treat entire text as one section
	if len(sections) == 0 && strings.TrimSpace(text) != "" {
		sections = append(sections, strings.TrimSpace(text))
	}

	return sections
}

const whyWhatTemplate = `
Why
===

{{ if ne .Why "" }}
{{ .Why }}
{{ else }}
 <!-- Describe what prompted you to make this change, link relevant resources: Asana tasks, Canny report, Slack discussions...etc -->
{{ end }}

What changed
============

{{ if ne .WhatChanged "" }}
{{ .WhatChanged }}
{{ else }}
<!-- Describe what changed to a level of detail that someone with no context with your PR could be able to review it -->
{{ end }}

Test plan
=========

{{ if ne .TestPlan "" }}
{{ .TestPlan }}
{{ else }}
<!--
  - You must provide a test plan that is detailed enough that any
    engineer at the company could confidently test your changes in
    staging and approve them on your behalf.
-->
{{ end }}

Rollout
=======

<!-- Describe any procedures or requirements needed to roll this out safely (or check the box below) -->

- [x] This is fully backward and forward compatible
`
