package vcs

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ejoffe/spr/config"
	"github.com/stretchr/testify/require"
)

func TestIsJJColocated_NoJJDir(t *testing.T) {
	dir := t.TempDir()
	require.False(t, IsJJColocated(dir))
}

func TestIsJJColocated_WithJJDir(t *testing.T) {
	dir := t.TempDir()
	err := os.Mkdir(filepath.Join(dir, ".jj"), 0755)
	require.NoError(t, err)
	require.True(t, IsJJColocated(dir))
}

func TestIsJJColocated_EmptyString(t *testing.T) {
	require.False(t, IsJJColocated(""))
}

func TestNewVCSOperations_GitRepo(t *testing.T) {
	cfg := config.EmptyConfig()
	dir := t.TempDir()
	gitmock := &mockRootDirOnly{rootDir: dir}
	ops := NewVCSOperations(cfg, gitmock)
	_, isGit := ops.(*GitOps)
	require.True(t, isGit, "should return GitOps for non-jj repo")
}

func TestNewVCSOperations_JJRepo(t *testing.T) {
	cfg := config.EmptyConfig()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".jj"), 0755)
	gitmock := &mockRootDirOnly{rootDir: dir}
	ops := NewVCSOperations(cfg, gitmock)
	_, isJj := ops.(*JjOps)
	require.True(t, isJj, "should return JjOps for jj-colocated repo")
}

func TestNewVCSOperations_JJRepo_NoJJFlag(t *testing.T) {
	cfg := config.EmptyConfig()
	cfg.User.NoJJ = true
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".jj"), 0755)
	gitmock := &mockRootDirOnly{rootDir: dir}
	ops := NewVCSOperations(cfg, gitmock)
	_, isGit := ops.(*GitOps)
	require.True(t, isGit, "should return GitOps when NoJJ is true even with .jj/ present")
}

// mockRootDirOnly implements git.GitInterface with only RootDir meaningful.
type mockRootDirOnly struct {
	rootDir string
}

func (m *mockRootDirOnly) GitWithEditor(args string, output *string, editorCmd string) error {
	return nil
}
func (m *mockRootDirOnly) Git(args string, output *string) error                             { return nil }
func (m *mockRootDirOnly) MustGit(args string, output *string)                               {}
func (m *mockRootDirOnly) RootDir() string                                                   { return m.rootDir }
func (m *mockRootDirOnly) DeleteRemoteBranch(ctx context.Context, branch string) error       { return nil }
