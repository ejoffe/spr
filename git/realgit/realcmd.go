package realgit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

func NewGitCmd() *gitcmd {
	initcmd := &gitcmd{}
	var rootdir string
	err := initcmd.Git("rev-parse --show-toplevel", &rootdir)
	if err != nil {
		panic(err)
	}
	rootdir = strings.TrimSpace(rootdir)

	return &gitcmd{
		rootdir: rootdir,
	}
}

type gitcmd struct {
	rootdir string
}

func (c *gitcmd) Git(argStr string, output *string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command

	log.Debug().Msg("git " + argStr)
	args := strings.Split(argStr, " ")
	cmd := exec.Command("git", args...)
	cmd.Dir = c.rootdir
	envVarsToDerive := []string{
		"SSH_AUTH_SOCK",
		"SSH_AGENT_PID",
		"HOME",
		"XDG_CONFIG_HOME",
	}
	cmd.Env = []string{"EDITOR=/usr/bin/true"}
	for _, env := range envVarsToDerive {
		envval := os.Getenv(env)
		if envval != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env, envval))
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
