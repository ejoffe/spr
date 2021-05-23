package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/spr/spr"
	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
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

// command line opts
type opts struct {
	Debug   bool `short:"d" long:"debug" description:"Show runtime debug info."`
	Merge   bool `short:"m" long:"merge" description:"Merge all mergeable pull requests."`
	Status  bool `short:"s" long:"status" description:"Show status of open pull requests."`
	Update  bool `short:"u" long:"update" description:"Update and create pull requests for unmerged commits in the stack."`
	Version bool `short:"v" long:"version" description:"Show version info."`
}

func main() {
	var opts opts
	_, err := flags.Parse(&opts)
	check(err)

	if opts.Version {
		fmt.Printf("spr version : %s : %s : %s\n", version, date, commit[:8])
		os.Exit(0)
	}

	ctx := context.Background()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Printf("GitHub OAuth Token Required\n")
		fmt.Printf("Make one at: https://%s/settings/tokens\n", "github.com")
		fmt.Printf("And set an env variable called GITHUB_TOKEN with it's value\n")
		os.Exit(-1)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := githubv4.NewClient(tc)

	config := &spr.Config{
		GitHubRepoOwner: "ejoffe",
		GitHubRepoName:  "apomelo",
		RequireChecks:   true,
		RequireApproval: false,
	}

	stackedpr := spr.NewStackedPR(config)
	if opts.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		stackedpr.DebugMode(true)
	}

	if opts.Update {
		stackedpr.UpdatePullRequests(ctx, client)
	} else if opts.Merge {
		stackedpr.MergePullRequests(ctx, client)
		stackedpr.UpdatePullRequests(ctx, client)
	} else if opts.Status {
		stackedpr.StatusPullRequests(ctx, client)
	} else {
		stackedpr.StatusPullRequests(ctx, client)
	}

	if opts.Debug {
		stackedpr.DebugPrintSummary()
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
