package template_custom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGit is a simple mock implementation of git.GitInterface for testing
type mockGit struct {
	rootDir string
}

func (m *mockGit) GitWithEditor(args string, output *string, editorCmd string) error {
	return nil
}

func (m *mockGit) Git(args string, output *string) error {
	return nil
}

func (m *mockGit) MustGit(args string, output *string) {
}

func (m *mockGit) RootDir() string {
	return m.rootDir
}

func TestTitle(t *testing.T) {
	repoConfig := &config.RepoConfig{}
	gitcmd := &mockGit{rootDir: "/tmp"}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := templatizer.Title(info, tt.commit)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatBody(t *testing.T) {
	repoConfig := &config.RepoConfig{
		ShowPrTitlesInStack: false,
	}
	gitcmd := &mockGit{rootDir: "/tmp"}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	tests := []struct {
		name     string
		commit   git.Commit
		stack    []*github.PullRequest
		contains []string
	}{
		{
			name: "single commit in stack",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Commit body",
			},
			stack: []*github.PullRequest{
				{Number: 1, Commit: git.Commit{CommitID: "commit1"}},
			},
			contains: []string{"Commit body"},
		},
		{
			name: "empty stack",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Commit body",
			},
			stack:    []*github.PullRequest{},
			contains: []string{"Commit body"},
		},
		{
			name: "multiple commits with body",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Commit body text",
			},
			stack: []*github.PullRequest{
				{Number: 1, Commit: git.Commit{CommitID: "commit1"}},
				{Number: 2, Commit: git.Commit{CommitID: "commit2"}},
			},
			contains: []string{
				"Commit body text",
				"---",
				"**Stack**:",
				"#1",
				"#2",
				"⚠️",
				"Part of a stack created by [spr]",
			},
		},
		{
			name: "multiple commits with empty body",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "",
			},
			stack: []*github.PullRequest{
				{Number: 1, Commit: git.Commit{CommitID: "commit1"}},
				{Number: 2, Commit: git.Commit{CommitID: "commit2"}},
			},
			contains: []string{
				"**Stack**:",
				"#1",
				"#2",
				"⚠️",
			},
		},
		{
			name: "stack with PR titles",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Commit body",
			},
			stack: []*github.PullRequest{
				{Number: 1, Title: "First PR", Commit: git.Commit{CommitID: "commit1"}},
				{Number: 2, Title: "Second PR", Commit: git.Commit{CommitID: "commit2"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templatizer.formatBody(tt.commit, tt.stack)

			for _, wantStr := range tt.contains {
				assert.Contains(t, result, wantStr, "Expected output to contain: %s", wantStr)
			}

			// For single commit or empty stack, body should be trimmed
			if len(tt.stack) <= 1 {
				assert.Equal(t, strings.TrimSpace(tt.commit.Body), result)
			}
		})
	}
}

func TestFormatBodyWithPRTitles(t *testing.T) {
	repoConfig := &config.RepoConfig{
		ShowPrTitlesInStack: true,
	}
	gitcmd := &mockGit{rootDir: "/tmp"}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	commit := git.Commit{
		Subject: "Test commit",
		Body:    "Commit body",
	}
	stack := []*github.PullRequest{
		{Number: 1, Title: "First PR", Commit: git.Commit{CommitID: "commit1"}},
		{Number: 2, Title: "Second PR", Commit: git.Commit{CommitID: "commit2"}},
	}

	result := templatizer.formatBody(commit, stack)

	assert.Contains(t, result, "First PR #1")
	assert.Contains(t, result, "Second PR #2")
}

func TestReadPRTemplate(t *testing.T) {
	// Create a temporary directory and file
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "pr_template.md")
	templateContent := "# PR Template\n\n<!-- INSERT_BODY -->\n\n## Additional Notes\n"
	
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	repoConfig := &config.RepoConfig{
		PRTemplatePath: "pr_template.md",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	result, err := templatizer.readPRTemplate()
	require.NoError(t, err)
	assert.Equal(t, templateContent, result)
}

func TestReadPRTemplateNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	
	repoConfig := &config.RepoConfig{
		PRTemplatePath: "nonexistent_template.md",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	_, err := templatizer.readPRTemplate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read template")
}

func TestReadPRTemplateWithSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "templates")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	
	templatePath := filepath.Join(subDir, "pr_template.md")
	templateContent := "Template in subdirectory"
	
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	repoConfig := &config.RepoConfig{
		PRTemplatePath: "templates/pr_template.md",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	result, err := templatizer.readPRTemplate()
	require.NoError(t, err)
	assert.Equal(t, templateContent, result)
}

func TestGetSectionOfPRTemplate(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		searchString string
		matchType    int
		expected     string
		expectError  bool
	}{
		{
			name:         "before match",
			text:         "Before<!-- INSERT -->After",
			searchString: "<!-- INSERT -->",
			matchType:    BeforeMatch,
			expected:     "Before",
			expectError:  false,
		},
		{
			name:         "after match",
			text:         "Before<!-- INSERT -->After",
			searchString: "<!-- INSERT -->",
			matchType:    AfterMatch,
			expected:     "After",
			expectError:  false,
		},
		{
			name:         "no match found",
			text:         "Some text without marker",
			searchString: "<!-- INSERT -->",
			matchType:    BeforeMatch,
			expectError:  true,
		},
		{
			name:         "multiple matches",
			text:         "Before<!-- INSERT -->Middle<!-- INSERT -->After",
			searchString: "<!-- INSERT -->",
			matchType:    BeforeMatch,
			expectError:  true,
		},
		{
			name:         "empty search string",
			text:         "Some text",
			searchString: "",
			matchType:    BeforeMatch,
			expectError:  true,
		},
		{
			name:         "match at start",
			text:         "<!-- START -->Rest of text",
			searchString: "<!-- START -->",
			matchType:    BeforeMatch,
			expected:     "",
			expectError:  false,
		},
		{
			name:         "match at end",
			text:         "Text before<!-- END -->",
			searchString: "<!-- END -->",
			matchType:    AfterMatch,
			expected:     "",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getSectionOfPRTemplate(tt.text, tt.searchString, tt.matchType)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestInsertBodyIntoPRTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "pr_template.md")
	prTemplate := "# PR Template\n\n<!-- START -->\n\n<!-- END -->\n\n## Additional Notes\n"
	
	err := os.WriteFile(templatePath, []byte(prTemplate), 0644)
	require.NoError(t, err)

	repoConfig := &config.RepoConfig{
		PRTemplatePath:        "pr_template.md",
		PRTemplateInsertStart: "<!-- START -->",
		PRTemplateInsertEnd:   "<!-- END -->",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	body := "This is the commit body"
	result, err := templatizer.insertBodyIntoPRTemplate(body, prTemplate, nil)
	require.NoError(t, err)

	expected := "# PR Template\n\n<!-- START -->\nThis is the commit body\n\n<!-- END -->\n\n## Additional Notes\n"
	assert.Equal(t, expected, result)
}

func TestInsertBodyIntoPRTemplateWithExistingPR(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "pr_template.md")
	prTemplate := "# PR Template\n\n<!-- START -->\n\n<!-- END -->\n"
	
	err := os.WriteFile(templatePath, []byte(prTemplate), 0644)
	require.NoError(t, err)

	repoConfig := &config.RepoConfig{
		PRTemplatePath:        "pr_template.md",
		PRTemplateInsertStart:  "<!-- START -->",
		PRTemplateInsertEnd:    "<!-- END -->",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	body := "Updated commit body"
	existingPRBody := "# PR Template\n\n<!-- START -->\nOld body\n\n<!-- END -->\n"
	
	result, err := templatizer.insertBodyIntoPRTemplate(body, prTemplate, &github.PullRequest{
		Body: existingPRBody,
	})
	require.NoError(t, err)

	// Should use existing PR body instead of template
	expected := "# PR Template\n\n<!-- START -->\nUpdated commit body\n\n<!-- END -->\n"
	assert.Equal(t, expected, result)
}

func TestInsertBodyIntoPRTemplateMissingStartMarker(t *testing.T) {
	tmpDir := t.TempDir()
	prTemplate := "# PR Template\n\n<!-- END -->\n"

	repoConfig := &config.RepoConfig{
		PRTemplatePath:       "pr_template.md",
		PRTemplateInsertStart: "<!-- START -->",
		PRTemplateInsertEnd:   "<!-- END -->",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	body := "Commit body"
	_, err := templatizer.insertBodyIntoPRTemplate(body, prTemplate, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PR template insert start")
}

func TestInsertBodyIntoPRTemplateMissingEndMarker(t *testing.T) {
	tmpDir := t.TempDir()
	prTemplate := "# PR Template\n\n<!-- START -->\n"

	repoConfig := &config.RepoConfig{
		PRTemplatePath:       "pr_template.md",
		PRTemplateInsertStart: "<!-- START -->",
		PRTemplateInsertEnd:   "<!-- END -->",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	body := "Commit body"
	_, err := templatizer.insertBodyIntoPRTemplate(body, prTemplate, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PR template insert end")
}

func TestInsertBodyIntoPRTemplateMultipleStartMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	prTemplate := "# PR Template\n\n<!-- START -->\n<!-- START -->\n<!-- END -->\n"

	repoConfig := &config.RepoConfig{
		PRTemplatePath:       "pr_template.md",
		PRTemplateInsertStart: "<!-- START -->",
		PRTemplateInsertEnd:   "<!-- END -->",
	}
	gitcmd := &mockGit{rootDir: tmpDir}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	body := "Commit body"
	_, err := templatizer.insertBodyIntoPRTemplate(body, prTemplate, nil)
	assert.Error(t, err)
}

func TestNewCustomTemplatizer(t *testing.T) {
	repoConfig := &config.RepoConfig{
		PRTemplatePath: "template.md",
	}
	gitcmd := &mockGit{rootDir: "/tmp"}

	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)
	assert.NotNil(t, templatizer)
	assert.Equal(t, repoConfig, templatizer.repoConfig)
	assert.Equal(t, gitcmd, templatizer.gitcmd)
}

func TestFormatBodyStackOrder(t *testing.T) {
	repoConfig := &config.RepoConfig{
		ShowPrTitlesInStack: false,
	}
	gitcmd := &mockGit{rootDir: "/tmp"}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	commit1 := git.Commit{CommitID: "commit1", Subject: "First"}
	commit2 := git.Commit{CommitID: "commit2", Subject: "Second"}
	commit3 := git.Commit{CommitID: "commit3", Subject: "Third"}

	commit := git.Commit{
		Subject: "Test",
		Body:    "Body text",
	}
	stack := []*github.PullRequest{
		{Number: 1, Commit: commit1},
		{Number: 2, Commit: commit2},
		{Number: 3, Commit: commit3},
	}

	result := templatizer.formatBody(commit, stack)

	// Stack should be in reverse order (3, 2, 1)
	idx3 := strings.Index(result, "#3")
	idx2 := strings.Index(result, "#2")
	idx1 := strings.Index(result, "#1")

	assert.Greater(t, idx3, -1)
	assert.Greater(t, idx2, -1)
	assert.Greater(t, idx1, -1)
	assert.True(t, idx3 < idx2, "#3 should come before #2")
	assert.True(t, idx2 < idx1, "#2 should come before #1")
}

func TestFormatBodyCurrentCommitIndicator(t *testing.T) {
	repoConfig := &config.RepoConfig{
		ShowPrTitlesInStack: false,
	}
	gitcmd := &mockGit{rootDir: "/tmp"}
	templatizer := NewCustomTemplatizer(repoConfig, gitcmd)

	commit1 := git.Commit{CommitID: "commit1", Subject: "First"}
	commit2 := git.Commit{CommitID: "commit2", Subject: "Second"}

	commit := commit2
	stack := []*github.PullRequest{
		{Number: 1, Commit: commit1},
		{Number: 2, Commit: commit2},
	}

	result := templatizer.formatBody(commit, stack)

	// Current commit should have arrow indicator
	assert.Contains(t, result, "#2 ⬅")
	assert.NotContains(t, result, "#1 ⬅")
}

