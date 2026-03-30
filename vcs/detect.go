package vcs

import (
	"os"
	"path/filepath"
)

// IsJJColocated returns true if the repository at rootDir is a jj-colocated
// repo (has both .jj/ and .git/ directories).
func IsJJColocated(rootDir string) bool {
	_, err := os.Stat(filepath.Join(rootDir, ".jj"))
	return err == nil
}
