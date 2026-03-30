package vcs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStringsFieldsSplitsTemplates demonstrates that strings.Fields
// (used by JjCmd.Jj) incorrectly splits jj template arguments containing
// spaces. Commands with templates must use JjArgs to preserve argument boundaries.
func TestStringsFieldsSplitsTemplates(t *testing.T) {
	template := `commit_id ++ "\x1f" ++ change_id ++ "\x1f" ++ empty ++ "\x1f" ++ description ++ "\x1e"`
	cmdStr := `log --no-graph --reversed --color=never -r "trunk()..@" -T '` + template + `'`

	fields := strings.Fields(cmdStr)

	// Find the -T flag
	tIdx := -1
	for i, f := range fields {
		if f == "-T" {
			tIdx = i
			break
		}
	}
	assert.NotEqual(t, -1, tIdx, "-T flag should be present")

	// strings.Fields splits the template — the next field is just "'commit_id", not the full template.
	// This proves that Jj() (which uses strings.Fields) cannot be used for commands with templates.
	assert.False(t, strings.Contains(fields[tIdx+1], "description"),
		"strings.Fields splits template args — commands with templates must use JjArgs instead")
}
