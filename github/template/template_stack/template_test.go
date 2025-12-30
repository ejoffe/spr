package template_stack

import (
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/stretchr/testify/assert"
)

func TestTitle(t *testing.T) {
	templatizer := NewStackTemplatizer(false)
	info := &github.GitHubInfo{}

	tests := []struct {
		name   string
		commit git.Commit
		want   string
	}{
		{
			name: "simple subject",
			commit: git.Commit{
				Subject: "Fix bug in authentication",
				Body:    "Some body text",
			},
			want: "Fix bug in authentication",
		},
		{
			name: "empty subject",
			commit: git.Commit{
				Subject: "",
				Body:    "Some body text",
			},
			want: "",
		},
		{
			name: "subject with special characters",
			commit: git.Commit{
				Subject: "Add feature: user authentication (WIP)",
				Body:    "Some body text",
			},
			want: "Add feature: user authentication (WIP)",
		},
		{
			name: "subject with newline",
			commit: git.Commit{
				Subject: "Fix bug\nwith newline",
				Body:    "Some body text",
			},
			want: "Fix bug\nwith newline",
		},
		{
			name: "long subject",
			commit: git.Commit{
				Subject: "This is a very long commit subject that might be used in some cases where developers write detailed commit messages",
				Body:    "Some body text",
			},
			want: "This is a very long commit subject that might be used in some cases where developers write detailed commit messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := templatizer.Title(info, tt.commit)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBody_EmptyStack(t *testing.T) {
	templatizer := NewStackTemplatizer(false)
	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{},
	}

	commit := git.Commit{
		Subject: "Test commit",
		Body:    "Commit body text",
	}

	result := templatizer.Body(info, commit, nil)

	// Should contain the commit body
	assert.Contains(t, result, "Commit body text")

	// Should contain the manual merge notice
	assert.Contains(t, result, "⚠️")
	assert.Contains(t, result, "Part of a stack created by [spr]")
	assert.Contains(t, result, "Do not merge manually using the UI")

	// Should end with the notice
	assert.True(t, strings.HasSuffix(result, "Do not merge manually using the UI - doing so may have unexpected results.*"))
}

func TestBody_WithStack_NoTitles(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit1 := git.Commit{
		CommitID: "commit1",
		Subject:  "First commit",
		Body:     "First body",
	}
	commit2 := git.Commit{
		CommitID: "commit2",
		Subject:  "Second commit",
		Body:     "Second body",
	}
	commit3 := git.Commit{
		CommitID: "commit3",
		Subject:  "Third commit",
		Body:     "Third body",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{
				Number: 1,
				Title:  "First commit",
				Commit: commit1,
			},
			{
				Number: 2,
				Title:  "Second commit",
				Commit: commit2,
			},
			{
				Number: 3,
				Title:  "Third commit",
				Commit: commit3,
			},
		},
	}

	// Test with commit2 (middle of stack)
	result := templatizer.Body(info, commit2, nil)

	// Should contain the commit body
	assert.Contains(t, result, "Second body")

	// Should contain stack markdown (in reverse order: 3, 2, 1)
	// Stack is formatted in reverse, so commit3 should be first
	assert.Contains(t, result, "#3")
	assert.Contains(t, result, "#2")
	assert.Contains(t, result, "#1")

	// Current commit (commit2) should have the arrow indicator
	assert.Contains(t, result, "#2 ⬅")

	// Other commits should not have the arrow
	// We need to check that #1 and #3 don't have the arrow
	lines := strings.Split(result, "\n")
	foundCurrent := false
	for _, line := range lines {
		if strings.Contains(line, "#2 ⬅") {
			foundCurrent = true
		}
		// Other PR numbers should not have the arrow
		if strings.Contains(line, "#1") && !strings.Contains(line, "#2") {
			assert.NotContains(t, line, "⬅", "PR #1 should not have arrow indicator")
		}
		if strings.Contains(line, "#3") && !strings.Contains(line, "#2") {
			assert.NotContains(t, line, "⬅", "PR #3 should not have arrow indicator")
		}
	}
	assert.True(t, foundCurrent, "Should find current commit with arrow indicator")

	// Should contain the manual merge notice
	assert.Contains(t, result, "⚠️")
	assert.Contains(t, result, "Part of a stack created by [spr]")
}

func TestBody_WithStack_WithTitles(t *testing.T) {
	templatizer := NewStackTemplatizer(true)

	commit1 := git.Commit{
		CommitID: "commit1",
		Subject:  "First commit",
		Body:     "First body",
	}
	commit2 := git.Commit{
		CommitID: "commit2",
		Subject:  "Second commit",
		Body:     "Second body",
	}
	commit3 := git.Commit{
		CommitID: "commit3",
		Subject:  "Third commit",
		Body:     "Third body",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{
				Number: 1,
				Title:  "First commit",
				Commit: commit1,
			},
			{
				Number: 2,
				Title:  "Second commit",
				Commit: commit2,
			},
			{
				Number: 3,
				Title:  "Third commit",
				Commit: commit3,
			},
		},
	}

	// Test with commit2 (middle of stack)
	result := templatizer.Body(info, commit2, nil)

	// Should contain the commit body
	assert.Contains(t, result, "Second body")

	// Should contain PR titles in the stack
	assert.Contains(t, result, "First commit #1")
	assert.Contains(t, result, "Second commit #2")
	assert.Contains(t, result, "Third commit #3")

	// Current commit should have the arrow
	assert.Contains(t, result, "Second commit #2 ⬅")

	// Should contain the manual merge notice
	assert.Contains(t, result, "⚠️")
}

func TestBody_StackOrder(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit1 := git.Commit{
		CommitID: "commit1",
		Subject:  "First",
		Body:     "Body 1",
	}
	commit2 := git.Commit{
		CommitID: "commit2",
		Subject:  "Second",
		Body:     "Body 2",
	}
	commit3 := git.Commit{
		CommitID: "commit3",
		Subject:  "Third",
		Body:     "Body 3",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 1, Commit: commit1},
			{Number: 2, Commit: commit2},
			{Number: 3, Commit: commit3},
		},
	}

	result := templatizer.Body(info, commit2, nil)

	// Stack should be in reverse order (3, 2, 1)
	// Find the stack section
	stackStart := strings.Index(result, "- #")
	assert.Greater(t, stackStart, -1, "Should find stack markdown")

	stackSection := (result)[stackStart:]

	// Check order: #3 should come before #2, #2 before #1
	idx3 := strings.Index(stackSection, "#3")
	idx2 := strings.Index(stackSection, "#2")
	idx1 := strings.Index(stackSection, "#1")

	assert.Greater(t, idx3, -1, "Should find #3")
	assert.Greater(t, idx2, -1, "Should find #2")
	assert.Greater(t, idx1, -1, "Should find #1")

	// Verify reverse order
	assert.True(t, idx3 < idx2, "#3 should come before #2")
	assert.True(t, idx2 < idx1, "#2 should come before #1")
}

func TestBody_CurrentCommitAtStart(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit1 := git.Commit{
		CommitID: "commit1",
		Subject:  "First",
		Body:     "Body 1",
	}
	commit2 := git.Commit{
		CommitID: "commit2",
		Subject:  "Second",
		Body:     "Body 2",
	}
	commit3 := git.Commit{
		CommitID: "commit3",
		Subject:  "Third",
		Body:     "Body 3",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 1, Commit: commit1},
			{Number: 2, Commit: commit2},
			{Number: 3, Commit: commit3},
		},
	}

	// Test with commit3 (last in stack, first in reverse order)
	result := templatizer.Body(info, commit3, nil)

	// Should have arrow on #3
	assert.Contains(t, result, "#3 ⬅")

	// Should not have arrow on others
	assert.NotContains(t, result, "#1 ⬅")
	assert.NotContains(t, result, "#2 ⬅")
}

func TestBody_CurrentCommitAtEnd(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit1 := git.Commit{
		CommitID: "commit1",
		Subject:  "First",
		Body:     "Body 1",
	}
	commit2 := git.Commit{
		CommitID: "commit2",
		Subject:  "Second",
		Body:     "Body 2",
	}
	commit3 := git.Commit{
		CommitID: "commit3",
		Subject:  "Third",
		Body:     "Body 3",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 1, Commit: commit1},
			{Number: 2, Commit: commit2},
			{Number: 3, Commit: commit3},
		},
	}

	// Test with commit1 (first in stack, last in reverse order)
	result := templatizer.Body(info, commit1, nil)

	// Should have arrow on #1
	assert.Contains(t, result, "#1 ⬅")

	// Should not have arrow on others
	assert.NotContains(t, result, "#2 ⬅")
	assert.NotContains(t, result, "#3 ⬅")
}

func TestBody_EmptyBody(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit := git.Commit{
		CommitID: "commit1",
		Subject:  "Test",
		Body:     "",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 1, Commit: commit},
		},
	}

	result := templatizer.Body(info, commit, nil)

	// Should still contain stack and notice
	assert.Contains(t, result, "#1")
	assert.Contains(t, result, "⚠️")
	assert.Contains(t, result, "Part of a stack created by [spr]")
}

func TestBody_SinglePRInStack(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit := git.Commit{
		CommitID: "commit1",
		Subject:  "Single commit",
		Body:     "Single body",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 42, Commit: commit},
		},
	}

	result := templatizer.Body(info, commit, nil)

	// Should contain the body
	assert.Contains(t, result, "Single body")

	// Should contain the PR number
	assert.Contains(t, result, "#42")

	// Should have arrow on current commit
	assert.Contains(t, result, "#42 ⬅")

	// Should contain the manual merge notice
	assert.Contains(t, result, "⚠️")
}

func TestBody_WithTitlesVsWithoutTitles(t *testing.T) {
	commit1 := git.Commit{
		CommitID: "commit1",
		Subject:  "First",
		Body:     "Body 1",
	}
	commit2 := git.Commit{
		CommitID: "commit2",
		Subject:  "Second",
		Body:     "Body 2",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 1, Title: "First PR", Commit: commit1},
			{Number: 2, Title: "Second PR", Commit: commit2},
		},
	}

	// Test without titles
	templatizerNoTitles := NewStackTemplatizer(false)
	resultNoTitles := templatizerNoTitles.Body(info, commit2, nil)

	// Should NOT contain PR titles
	assert.NotContains(t, resultNoTitles, "First PR")
	assert.NotContains(t, resultNoTitles, "Second PR")
	// Should only contain numbers
	assert.Contains(t, resultNoTitles, "- #1")
	assert.Contains(t, resultNoTitles, "- #2")

	// Test with titles
	templatizerWithTitles := NewStackTemplatizer(true)
	resultWithTitles := templatizerWithTitles.Body(info, commit2, nil)

	// Should contain PR titles
	assert.Contains(t, resultWithTitles, "First PR #1")
	assert.Contains(t, resultWithTitles, "Second PR #2")
}

func TestBody_Structure(t *testing.T) {
	templatizer := NewStackTemplatizer(false)

	commit := git.Commit{
		CommitID: "commit1",
		Subject:  "Test",
		Body:     "Commit body content",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{Number: 1, Commit: commit},
		},
	}

	result := templatizer.Body(info, commit, nil)

	// Verify structure: body + \n\n + stack + \n\n + notice
	// The body should come first
	assert.True(t, strings.HasPrefix(result, "Commit body content"))

	// Should have stack markdown
	assert.Contains(t, result, "- #1")

	// Should end with manual merge notice
	assert.True(t, strings.HasSuffix(result, "Do not merge manually using the UI - doing so may have unexpected results.*"))
}

func TestBody_RealWorldExample(t *testing.T) {
	templatizer := NewStackTemplatizer(true)

	commit1 := git.Commit{
		CommitID: "abc123",
		Subject:  "Add authentication middleware",
		Body:     "Implemented JWT token validation",
	}
	commit2 := git.Commit{
		CommitID: "def456",
		Subject:  "Add user login endpoint",
		Body:     "Created POST /login endpoint",
	}
	commit3 := git.Commit{
		CommitID: "ghi789",
		Subject:  "Add user registration",
		Body:     "Created POST /register endpoint",
	}

	info := &github.GitHubInfo{
		PullRequests: []*github.PullRequest{
			{
				Number: 10,
				Title:  "Add authentication middleware",
				Commit: commit1,
			},
			{
				Number: 11,
				Title:  "Add user login endpoint",
				Commit: commit2,
			},
			{
				Number: 12,
				Title:  "Add user registration",
				Commit: commit3,
			},
		},
	}

	// Test with middle commit
	result := templatizer.Body(info, commit2, nil)

	// Should contain commit body
	assert.Contains(t, result, "Created POST /login endpoint")

	// Should contain all PRs in reverse order with titles
	assert.Contains(t, result, "Add user registration #12")
	assert.Contains(t, result, "Add user login endpoint #11 ⬅")
	assert.Contains(t, result, "Add authentication middleware #10")

	// Should contain manual merge notice
	assert.Contains(t, result, "⚠️")
	assert.Contains(t, result, "Part of a stack created by [spr]")
}

func TestNewStackTemplatizer(t *testing.T) {
	// Test that constructor works correctly
	templatizer1 := NewStackTemplatizer(false)
	assert.NotNil(t, templatizer1)

	templatizer2 := NewStackTemplatizer(true)
	assert.NotNil(t, templatizer2)

	// They should be different instances
	assert.NotEqual(t, templatizer1, templatizer2)
}

func TestFormatPullRequestBody(t *testing.T) {
	simpleCommit := git.Commit{
		CommitID:   "abc123",
		CommitHash: "abcdef123456",
	}
	descriptiveCommit := git.Commit{
		CommitID:   "def456",
		CommitHash: "ghijkl7890",
		Body: `This body describes my nice PR.
It even includes some **markdown** formatting.`}

	tests := []struct {
		name     string
		commit   git.Commit
		info     *github.GitHubInfo
		expected string
	}{
		{
			name:   "EmptyStack",
			commit: git.Commit{},
			info: &github.GitHubInfo{
				PullRequests: []*github.PullRequest{},
			},
			expected: `
---
**Stack**:
---
⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
		},
		{
			name:   "SinglePRStack",
			commit: descriptiveCommit,
			info: &github.GitHubInfo{
				PullRequests: []*github.PullRequest{
					{Number: 2, Commit: descriptiveCommit},
				},
			},
			expected: `This body describes my nice PR.
It even includes some **markdown** formatting.
---
**Stack**:
- #2 ⬅
---
⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
		},
		{
			name: "TwoPRStack",
			info: &github.GitHubInfo{
				PullRequests: []*github.PullRequest{
					{Number: 1, Commit: simpleCommit},
					{Number: 2, Commit: descriptiveCommit},
				},
			},
			expected: `This body describes my nice PR.
It even includes some **markdown** formatting.
---
**Stack**:
- #2 ⬅
- #1
---
⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
			commit: descriptiveCommit,
		},
	}

	templatizer := NewStackTemplatizer(false)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := templatizer.Body(tc.info, tc.commit, nil)
			if body != tc.expected {
				t.Fatalf("expected: '%v', actual: '%v'", tc.expected, body)
			}
		})
	}
}

/*
func TestFormatPullRequestBody_ShowPrTitle(t *testing.T) {
	simpleCommit := git.Commit{
		CommitID:   "abc123",
		CommitHash: "abcdef123456",
	}
	descriptiveCommit := git.Commit{
		CommitID:   "def456",
		CommitHash: "ghijkl7890",
		Body: `This body describes my nice PR.
It even includes some **markdown** formatting.`}

	tests := []struct {
		description string
		commit      git.Commit
		stack       []*github.PullRequest
	}{
		{
			description: "",
			commit:      git.Commit{},
			stack:       []*github.PullRequest{},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 2, Commit: descriptiveCommit},
			},
		},
		{
			description: `This body describes my nice PR.
It even includes some **markdown** formatting.

---

**Stack**:
- Title B #2 ⬅
- Title A #1


⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*`,
			commit: descriptiveCommit,
			stack: []*github.PullRequest{
				{Number: 1, Commit: simpleCommit, Title: "Title A"},
				{Number: 2, Commit: descriptiveCommit, Title: "Title B"},
			},
		},
	}

	for _, tc := range tests {
		body := formatBody(tc.commit, tc.stack, true)
		if body != tc.description {
			t.Fatalf("expected: '%v', actual: '%v'", tc.description, body)
		}
	}
}

func TestInsertBodyIntoPRTemplateHappyPath(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		pullRequestTemplate string
		repo                *config.RepoConfig
		pr                  *github.PullRequest
		expected            string
	}{
		{
			name: "create PR",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr: nil,
			expected: `
## Related Issues
<!--- Add any related issues here -->

## Description
inserted body

## Checklist
- [ ] My code follows the style guidelines of this project`,
		},
		{
			name: "update PR",
			body: "updated description",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr: &github.PullRequest{
				Body: `
## Related Issues
* Issue #1234

## Description
original description

## Checklist
- [x] My code follows the style guidelines of this project`,
			},
			expected: `
## Related Issues
* Issue #1234

## Description
updated description

## Checklist
- [x] My code follows the style guidelines of this project`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := insertBodyIntoPRTemplate(tt.body, tt.pullRequestTemplate, tt.repo, tt.pr)
			if body != tt.expected {
				t.Fatalf("expected: '%v', actual: '%v'", tt.expected, body)
			}
		})
	}
}

func TestInsertBodyIntoPRTemplateErrors(t *testing.T) {
	tests := []struct {
		name                string
		body                string
		pullRequestTemplate string
		repo                *config.RepoConfig
		pr                  *github.PullRequest
		expected            string
	}{
		{
			name: "no match insert start",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "does not exist",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr:       nil,
			expected: "no matches found: PR template insert start",
		},
		{
			name: "no match insert end",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here -->

## Description
<!--- Describe your changes in detail -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "does not exist",
			},
			pr:       nil,
			expected: "no matches found: PR template insert end",
		},
		{
			name: "multiple many matches insert start",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here duplicate -->

## Description
<!--- Describe your changes in detail duplicate -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "duplicate",
				PRTemplateInsertEnd:   "## Checklist",
			},
			pr:       nil,
			expected: "multiple matches found: PR template insert start",
		},
		{
			name: "multiple many matches insert end",
			body: "inserted body",
			pullRequestTemplate: `
## Related Issues
<!--- Add any related issues here duplicate -->

## Description
<!--- Describe your changes in detail duplicate -->

## Checklist
- [ ] My code follows the style guidelines of this project`,
			repo: &config.RepoConfig{
				PRTemplateInsertStart: "## Description",
				PRTemplateInsertEnd:   "duplicate",
			},
			pr:       nil,
			expected: "multiple matches found: PR template insert end",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := insertBodyIntoPRTemplate(tt.body, tt.pullRequestTemplate, tt.repo, tt.pr)
			if !strings.Contains(err.Error(), tt.expected) {
				t.Fatalf("expected: '%v', actual: '%v'", tt.expected, err.Error())
			}
		})
	}
}
*/
