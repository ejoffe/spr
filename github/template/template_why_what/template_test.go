package template_why_what

import (
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/stretchr/testify/assert"
)

func TestTitle(t *testing.T) {
	templatizer := &WhyWhatTemplatizer{}
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

func TestBody(t *testing.T) {
	templatizer := &WhyWhatTemplatizer{}
	info := &github.GitHubInfo{}

	tests := []struct {
		name        string
		commit      git.Commit
		want        string
		contains    []string // strings that should be in the output
		notContains []string // strings that should NOT be in the output
	}{
		{
			name: "all three sections provided",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "This is why we made the change\n\nThis is what changed\n\nThis is the test plan",
			},
			contains: []string{
				"Why\n===",
				"This is why we made the change",
				"What changed\n============",
				"This is what changed",
				"Test plan\n=========",
				"This is the test plan",
				"Rollout\n=======",
				"- [x] This is fully backward and forward compatible",
			},
			notContains: []string{
				"Describe what prompted you to make this change",
				"Describe what changed to a level of detail",
			},
		},
		{
			name: "only why section provided",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "This is why we made the change",
			},
			contains: []string{
				"Why\n===",
				"This is why we made the change",
				"What changed\n============",
				"Describe what changed to a level of detail",
				"Test plan\n=========",
				"You must provide a test plan",
			},
		},
		{
			name: "why and what changed sections provided",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "This is why we made the change\n\nThis is what changed",
			},
			contains: []string{
				"Why\n===",
				"This is why we made the change",
				"What changed\n============",
				"This is what changed",
				"Test plan\n=========",
				"You must provide a test plan",
			},
			notContains: []string{
				"Describe what prompted you to make this change",
				"Describe what changed to a level of detail",
			},
		},
		{
			name: "empty body",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "",
			},
			contains: []string{
				"Why\n===",
				"Describe what prompted you to make this change",
				"What changed\n============",
				"Describe what changed to a level of detail",
				"Test plan\n=========",
				"You must provide a test plan",
			},
		},
		{
			name: "body with only whitespace",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "   \n\n  \n  ",
			},
			contains: []string{
				"Why\n===",
				"Describe what prompted you to make this change",
			},
		},
		{
			name: "sections with extra whitespace",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "  Why section with spaces  \n\n  What changed section  \n\n  Test plan section  ",
			},
			contains: []string{
				"Why section with spaces",
				"What changed section",
				"Test plan section",
			},
		},
		{
			name: "multiple empty lines between sections",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Why section\n\n\n\nWhat changed section\n\n\n\nTest plan section",
			},
			contains: []string{
				"Why section",
				"What changed section",
				"Test plan section",
			},
		},
		{
			name: "sections with newlines within them",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Why section\nwith multiple\nlines\n\nWhat changed\nwith multiple\nlines\n\nTest plan\nwith multiple\nlines",
			},
			contains: []string{
				"Why section\nwith multiple\nlines",
				"What changed\nwith multiple\nlines",
				"Test plan\nwith multiple\nlines",
			},
		},
		{
			name: "only second section (what changed) provided",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "\n\nWhat changed section",
			},
			contains: []string{
				"Why\n===",
				"What changed section", // First non-empty section goes to Why
				"What changed\n============",
				"Describe what changed to a level of detail", // What Changed is empty, shows default
			},
		},
		{
			name: "only third section (test plan) provided",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "\n\n\nTest plan section",
			},
			contains: []string{
				"Why\n===",
				"Test plan section", // First non-empty section goes to Why
				"What changed\n============",
				"Describe what changed to a level of detail", // What Changed is empty, shows default
				"Test plan\n=========",
				"You must provide a test plan", // Test Plan is empty, shows default
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := templatizer.Body(info, tt.commit, nil)

			// Check that all required strings are present
			for _, wantStr := range tt.contains {
				assert.Contains(t, got, wantStr, "Expected output to contain: %s", wantStr)
			}

			// Check that excluded strings are not present
			for _, notWantStr := range tt.notContains {
				assert.NotContains(t, got, notWantStr, "Expected output to NOT contain: %s", notWantStr)
			}

			// Verify the structure is correct
			assert.Contains(t, got, "Why\n===")
			assert.Contains(t, got, "What changed\n============")
			assert.Contains(t, got, "Test plan\n=========")
			assert.Contains(t, got, "Rollout\n=======")
			assert.Contains(t, got, "- [x] This is fully backward and forward compatible")
		})
	}
}

func TestSplitByEmptyLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single section",
			input:    "Single section text",
			expected: []string{"Single section text"},
		},
		{
			name:     "single section with whitespace",
			input:    "  Single section text  ",
			expected: []string{"Single section text"},
		},
		{
			name:     "two sections",
			input:    "First section\n\nSecond section",
			expected: []string{"First section", "Second section"},
		},
		{
			name:     "three sections",
			input:    "First section\n\nSecond section\n\nThird section",
			expected: []string{"First section", "Second section", "Third section"},
		},
		{
			name:     "sections with leading/trailing whitespace",
			input:    "  First section  \n\n  Second section  \n\n  Third section  ",
			expected: []string{"First section", "Second section", "Third section"},
		},
		{
			name:     "multiple empty lines between sections",
			input:    "First section\n\n\n\nSecond section",
			expected: []string{"First section", "Second section"}, // Empty sections filtered out
		},
		{
			name:     "sections with internal newlines",
			input:    "First section\nwith multiple\nlines\n\nSecond section\nwith multiple\nlines",
			expected: []string{"First section\nwith multiple\nlines", "Second section\nwith multiple\nlines"},
		},
		{
			name:     "only whitespace",
			input:    "   \n\n  \n  ",
			expected: []string{}, // All empty sections filtered out
		},
		{
			name:     "empty lines at start and end",
			input:    "\n\nFirst section\n\nSecond section\n\n",
			expected: []string{"First section", "Second section"}, // Empty sections filtered out
		},
		{
			name:     "single newline (not empty line)",
			input:    "First section\nSecond section",
			expected: []string{"First section\nSecond section"},
		},
		{
			name:     "mixed single and double newlines",
			input:    "First section\nSecond line\n\nThird section",
			expected: []string{"First section\nSecond line", "Third section"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitByEmptyLines(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBodyTemplateStructure(t *testing.T) {
	// This test ensures the template always produces the correct structure
	templatizer := &WhyWhatTemplatizer{}
	info := &github.GitHubInfo{}

	commit := git.Commit{
		Subject: "Test",
		Body:    "Why section\n\nWhat changed section\n\nTest plan section",
	}

	result := templatizer.Body(info, commit, nil)

	// Verify sections appear in correct order
	whyIndex := strings.Index(result, "Why\n===")
	whatIndex := strings.Index(result, "What changed\n============")
	testIndex := strings.Index(result, "Test plan\n=========")
	rolloutIndex := strings.Index(result, "Rollout\n=======")

	assert.Greater(t, whyIndex, -1, "Why section should be present")
	assert.Greater(t, whatIndex, whyIndex, "What changed should come after Why")
	assert.Greater(t, testIndex, whatIndex, "Test plan should come after What changed")
	assert.Greater(t, rolloutIndex, testIndex, "Rollout should come after Test plan")
}

func TestBodyWithRealWorldExamples(t *testing.T) {
	templatizer := &WhyWhatTemplatizer{}
	info := &github.GitHubInfo{}

	tests := []struct {
		name   string
		commit git.Commit
	}{
		{
			name: "real commit message format",
			commit: git.Commit{
				Subject: "Add user authentication",
				Body: `We need to add authentication to secure the API endpoints.

Added JWT token validation middleware and user login endpoint.

Test by:
1. Start the server
2. Try to access protected endpoint without token
3. Login to get token
4. Access protected endpoint with token`,
			},
		},
		{
			name: "commit with only why",
			commit: git.Commit{
				Subject: "Fix critical bug",
				Body:    "Users reported that the app crashes when clicking the submit button.",
			},
		},
		{
			name: "commit with markdown-like content",
			commit: git.Commit{
				Subject: "Update documentation",
				Body: `Documentation was outdated.

- Updated API docs
- Added examples
- Fixed typos

Manual review of the docs.`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templatizer.Body(info, tt.commit, nil)

			// Should always contain the required sections
			assert.Contains(t, result, "Why\n===")
			assert.Contains(t, result, "What changed\n============")
			assert.Contains(t, result, "Test plan\n=========")
			assert.Contains(t, result, "Rollout\n=======")

			// Should not be empty
			assert.NotEmpty(t, result)

			// Should contain at least some content from the commit body
			if tt.commit.Body != "" {
				// Extract first line of body (likely to be in Why section)
				firstLine := strings.Split(tt.commit.Body, "\n")[0]
				if firstLine != "" {
					assert.Contains(t, result, firstLine)
				}
			}
		})
	}
}
