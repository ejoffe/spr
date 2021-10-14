package realgit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ejoffe/spr/config"
	"github.com/rs/zerolog/log"
)

// NewGitCmd returns a new git cmd instance
func NewGitCmd(cfg *config.Config) *gitcmd {
	initcmd := &gitcmd{
		config: cfg,
	}
	var rootdir string
	err := initcmd.Git("rev-parse --show-toplevel", &rootdir)
	if err != nil {
		panic(err)
	}
	rootdir = strings.TrimSpace(rootdir)

	return &gitcmd{
		config:  cfg,
		rootdir: rootdir,
	}
}

type gitcmd struct {
	config  *config.Config
	rootdir string
}

func (c *gitcmd) Git(argStr string, output *string) error {
	return c.GitWithEditor(argStr, output, "/usr/bin/true")
}

func (c *gitcmd) GitWithEditor(argStr string, output *string, editorCmd string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command

	log.Debug().Msg("git " + argStr)
	if c.config.User.LogGitCommands {
		fmt.Printf("> git %s\n", argStr)
	}
	args := strings.Split(argStr, " ")
	cmd := exec.Command("git", args...)
	cmd.Dir = c.rootdir

	cmd.Env = []string{fmt.Sprintf("EDITOR=%s", editorCmd)}
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)

		if parts[1] != "" && strings.ToUpper(parts[0]) != "EDITOR" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", parts[0], parts[1]))
		}
	}

	if output != nil {
		out, err := cmd.CombinedOutput()
		*output = strings.TrimSpace(string(out))
		if err != nil {
			fmt.Fprintf(os.Stderr, "git error: %s", string(out))
			return err
		}
	} else {
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "git error: %s", string(out))
			return err
		}
	}
	return nil
}

func (c *gitcmd) RootDir() string {
	return c.rootdir
}
