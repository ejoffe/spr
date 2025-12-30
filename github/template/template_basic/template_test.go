package template_basic

import (
	"strings"
	"testing"

	"github.com/ejoffe/spr/git"
	"github.com/ejoffe/spr/github"
	"github.com/stretchr/testify/assert"
)

func TestTitle(t *testing.T) {
	templatizer := &BasicTemplatizer{}
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

func TestBody(t *testing.T) {
	templatizer := &BasicTemplatizer{}
	info := &github.GitHubInfo{}

	tests := []struct {
		name         string
		commit       git.Commit
		wantContains []string // strings that should be in the output
		wantSuffix   string   // expected suffix (manual merge notice)
	}{
		{
			name: "empty body",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "",
			},
			wantContains: []string{
				"⚠️",
				"Part of a stack created by [spr]",
				"Do not merge manually using the UI",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "simple body",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "This is a simple commit body",
			},
			wantContains: []string{
				"This is a simple commit body",
				"⚠️",
				"Part of a stack created by [spr]",
				"Do not merge manually using the UI",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "body with newlines",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Line 1\nLine 2\nLine 3",
			},
			wantContains: []string{
				"Line 1",
				"Line 2",
				"Line 3",
				"⚠️",
				"Part of a stack created by [spr]",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "body with multiple paragraphs",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "First paragraph\n\nSecond paragraph\n\nThird paragraph",
			},
			wantContains: []string{
				"First paragraph",
				"Second paragraph",
				"Third paragraph",
				"⚠️",
				"Part of a stack created by [spr]",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "body with trailing newline",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Commit body with trailing newline\n",
			},
			wantContains: []string{
				"Commit body with trailing newline",
				"⚠️",
				"Part of a stack created by [spr]",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "body with markdown",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "# Header\n\n- List item 1\n- List item 2\n\n**Bold text**",
			},
			wantContains: []string{
				"# Header",
				"List item 1",
				"List item 2",
				"**Bold text**",
				"⚠️",
				"Part of a stack created by [spr]",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "body with special characters",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Body with special chars: @#$%^&*()[]{}|\\/<>?",
			},
			wantContains: []string{
				"Body with special chars: @#$%^&*()[]{}|\\/<>?",
				"⚠️",
				"Part of a stack created by [spr]",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
		{
			name: "body with unicode characters",
			commit: git.Commit{
				Subject: "Test commit",
				Body:    "Body with unicode: 你好世界 🌍 🚀",
			},
			wantContains: []string{
				"Body with unicode: 你好世界 🌍 🚀",
				"⚠️",
				"Part of a stack created by [spr]",
			},
			wantSuffix: "\n\n⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := templatizer.Body(info, tt.commit, nil)

			// Verify all expected strings are present
			for _, wantStr := range tt.wantContains {
				assert.Contains(t, got, wantStr, "Expected output to contain: %s", wantStr)
			}

			// Verify the manual merge notice is appended correctly
			assert.True(t, strings.HasSuffix(got, tt.wantSuffix),
				"Expected body to end with manual merge notice. Got suffix: %s",
				got[max(0, len(got)-len(tt.wantSuffix)):])

			// Verify the original body content is preserved at the start
			if tt.commit.Body != "" {
				// The body should start with the original commit body
				expectedStart := tt.commit.Body
				// If body doesn't end with newline, the notice adds \n\n, so we need to account for that
				if !strings.HasSuffix(tt.commit.Body, "\n") {
					assert.True(t, strings.HasPrefix(got, expectedStart),
						"Expected body to start with original commit body")
				}
			}
		})
	}
}

func TestBodyManualMergeNoticeFormat(t *testing.T) {
	templatizer := &BasicTemplatizer{}
	info := &github.GitHubInfo{}

	commit := git.Commit{
		Subject: "Test commit",
		Body:    "Test body",
	}

	result := templatizer.Body(info, commit, nil)

	// Verify the exact format of the manual merge notice
	expectedNotice := "⚠️ *Part of a stack created by [spr](https://github.com/ejoffe/spr). Do not merge manually using the UI - doing so may have unexpected results.*"

	// Should end with the notice
	assert.True(t, strings.HasSuffix(result, "\n\n"+expectedNotice))

	// Should contain the warning emoji
	assert.Contains(t, result, "⚠️")

	// Should contain the spr link
	assert.Contains(t, result, "[spr](https://github.com/ejoffe/spr)")

	// Should contain the warning message
	assert.Contains(t, result, "Do not merge manually using the UI")
}

func TestBodyPreservesOriginalContent(t *testing.T) {
	templatizer := &BasicTemplatizer{}
	info := &github.GitHubInfo{}

	testCases := []struct {
		name string
		body string
	}{
		{
			name: "preserves exact body content",
			body: "Exact body content to preserve",
		},
		{
			name: "preserves body with whitespace",
			body: "  Body with   whitespace  ",
		},
		{
			name: "preserves body with newlines",
			body: "Line 1\nLine 2\n\nLine 3",
		},
		{
			name: "preserves empty body",
			body: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			commit := git.Commit{
				Subject: "Test",
				Body:    tc.body,
			}

			result := templatizer.Body(info, commit, nil)

			// The result should contain the original body (if not empty)
			if tc.body != "" {
				// For non-empty bodies, the original content should appear before the notice
				noticeStart := strings.Index(result, "⚠️")
				if noticeStart > 0 {
					bodyPart := (result)[:noticeStart]
					// Remove the trailing \n\n that separates body from notice
					bodyPart = strings.TrimSuffix(bodyPart, "\n\n")
					assert.Equal(t, tc.body, bodyPart, "Original body content should be preserved")
				}
			} else {
				// For empty body, result should start with the notice (after \n\n)
				assert.True(t, strings.HasPrefix(result, "\n\n⚠️") || strings.HasPrefix(result, "⚠️"))
			}
		})
	}
}

func TestBodyWithRealWorldExamples(t *testing.T) {
	templatizer := &BasicTemplatizer{}
	info := &github.GitHubInfo{}

	tests := []struct {
		name   string
		commit git.Commit
	}{
		{
			name: "typical feature commit",
			commit: git.Commit{
				Subject: "Add user authentication",
				Body:    "Implemented JWT-based authentication for API endpoints. Users can now login and receive tokens.",
			},
		},
		{
			name: "bug fix commit",
			commit: git.Commit{
				Subject: "Fix memory leak in cache",
				Body:    "Fixed issue where cache entries were not being properly cleaned up, causing memory to grow over time.",
			},
		},
		{
			name: "documentation commit",
			commit: git.Commit{
				Subject: "Update README",
				Body:    "Added installation instructions and usage examples.",
			},
		},
		{
			name: "multi-line detailed commit",
			commit: git.Commit{
				Subject: "Refactor database layer",
				Body: `This commit refactors the database layer to improve performance.

Changes:
- Added connection pooling
- Optimized query execution
- Added caching layer

Performance improvements:
- 50% reduction in query time
- 30% reduction in memory usage`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templatizer.Body(info, tt.commit, nil)

			// Should contain the original body content
			assert.Contains(t, result, tt.commit.Body)

			// Should contain the manual merge notice
			assert.Contains(t, result, "⚠️")
			assert.Contains(t, result, "Part of a stack created by [spr]")
			assert.Contains(t, result, "Do not merge manually using the UI")

			// Should not be empty
			assert.NotEmpty(t, result)

			// Should end with the notice
			assert.True(t, strings.HasSuffix(result, "Do not merge manually using the UI - doing so may have unexpected results.*"))
		})
	}
}

// Helper function for max (since Go 1.18+ has it in math package, but for compatibility)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
