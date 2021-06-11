package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/github/githubclient"
	"github.com/ejoffe/spr/spr"
	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version = "dev"
	commit  = "dversion"
	date    = "unknown"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// command line options
type opts struct {
	Debug   bool `short:"d" long:"debug" description:"Show runtime debug info."`
	Merge   bool `short:"m" long:"merge" description:"Merge all mergeable pull requests."`
	Status  bool `short:"s" long:"status" description:"Show status of open pull requests."`
	Update  bool `short:"u" long:"update" description:"Update and create pull requests for unmerged commits in the stack."`
	Version bool `short:"v" long:"version" description:"Show version info."`
}

func main() {
	//  parse command line options
	var opts opts
	parser := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	_, err := parser.Parse()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if opts.Version {
		fmt.Printf("spr version : %s : %s : %s\n", version, date, commit[:8])
		os.Exit(0)
	}

	//  check that we are inside a git dir
	err = spr.SanityCheck()
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	// parse configuration
	cfg := config.Config{}
	rake.LoadSources(&cfg,
		rake.DefaultSource(),
		config.GitHubRemoteSource(&cfg),
		rake.YamlFileSource(config.ConfigFilePath()),
		rake.YamlFileWriter(config.ConfigFilePath()),
	)

	if opts.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		rake.LoadSources(&cfg, rake.DebugWriter(os.Stdout))
	}

	ctx := context.Background()
	client := githubclient.NewGitHubClient(ctx, &cfg)
	stackedpr := spr.NewStackedPR(&cfg, client, os.Stdout, opts.Debug)

	if opts.Update {
		stackedpr.UpdatePullRequests(ctx)
	} else if opts.Merge {
		stackedpr.MergePullRequests(ctx)
		stackedpr.UpdatePullRequests(ctx)
	} else if opts.Status {
		stackedpr.StatusPullRequests(ctx)
	} else {
		stackedpr.StatusPullRequests(ctx)
	}

	if opts.Debug {
		stackedpr.DebugPrintSummary()
	}
}
