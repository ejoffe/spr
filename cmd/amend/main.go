package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/spr"
	"github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version = "dev"
	commit  = "dversion"
	date    = "unknown"
)

// command line opts
type opts struct {
	Debug   bool `short:"d" long:"debug" description:"Show verbose debug info."`
	Version bool `short:"v" long:"version" description:"Show version."`
}

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func main() {
	var opts opts
	_, err := flags.Parse(&opts)
	check(err)

	if opts.Version {
		fmt.Printf("amend version : %s : %s : %s\n", version, date, commit[:8])
		os.Exit(0)
	}

	if opts.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	gitcmd := realgit.NewGitCmd(config.DefaultConfig())
	//  check that we are inside a git dir
	var output string
	err = gitcmd.Git("status --porcelain", &output)
	if err != nil {
		fmt.Println(output)
		fmt.Println(err)
		os.Exit(2)
	}

	cfg := config.ParseConfig(gitcmd)
	gitcmd = realgit.NewGitCmd(cfg)
	ctx := context.Background()

	sd := spr.NewStackedPR(cfg, nil, gitcmd)
	sd.AmendCommit(ctx)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
