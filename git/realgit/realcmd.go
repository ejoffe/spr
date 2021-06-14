package realgit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

func Cmd(argStr string, output *string) error {
	// runs a git command
	//  if output is not nil it will be set to the output of the command
	log.Debug().Msg("git " + argStr)
	args := strings.Split(argStr, " ")
	cmd := exec.Command("git", args...)
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

func RootDir() string {
	var rootdir string
	err := Cmd("rev-parse --show-toplevel", &rootdir)
	if err != nil {
		panic(err)
	}
	rootdir = strings.TrimSpace(rootdir)
	return rootdir
}
