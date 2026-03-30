package vcs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

// JjCmd executes jj commands, mirroring the git/realgit/realcmd.go pattern.
type JjCmd struct {
	rootdir string
}

// NewJjCmd creates a new jj command executor.
func NewJjCmd(rootDir string) *JjCmd {
	return &JjCmd{rootdir: rootDir}
}

// JjInterface abstracts jj command execution for testing.
type JjInterface interface {
	Jj(args string, output *string) error
	MustJj(args string, output *string)
	JjArgs(args []string, output *string) error
}

// Jj executes a jj command with the given arguments.
func (c *JjCmd) Jj(args string, output *string) error {
	log.Debug().Msgf("jj %s", args)
	cmdArgs := strings.Fields(args)
	cmd := exec.Command("jj", cmdArgs...)
	cmd.Dir = c.rootdir
	cmd.Env = append(os.Environ(), "JJ_CONFIG=")

	out, err := cmd.CombinedOutput()
	if output != nil {
		*output = strings.TrimRight(string(out), "\n")
	}
	if err != nil {
		return fmt.Errorf("jj %s: %w\n%s", args, err, string(out))
	}
	return nil
}

// MustJj executes a jj command and panics on error.
func (c *JjCmd) MustJj(args string, output *string) {
	err := c.Jj(args, output)
	if err != nil {
		panic(err)
	}
}

// JjArgs executes a jj command with pre-split arguments (for messages with spaces).
func (c *JjCmd) JjArgs(args []string, output *string) error {
	log.Debug().Msgf("jj %v", args)
	cmd := exec.Command("jj", args...)
	cmd.Dir = c.rootdir
	cmd.Env = append(os.Environ(), "JJ_CONFIG=")

	out, err := cmd.CombinedOutput()
	if output != nil {
		*output = strings.TrimRight(string(out), "\n")
	}
	if err != nil {
		return fmt.Errorf("jj %v: %w\n%s", args, err, string(out))
	}
	return nil
}
