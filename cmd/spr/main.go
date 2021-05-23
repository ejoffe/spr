package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/spr/stackediff"
	flags "github.com/jessevdk/go-flags"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.With().Caller().Logger().Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// command line opts
type opts struct {
	Amend  bool `short:"a" long:"amend" description:"Amends a chosen git commit with the current staged changes."`
	Debug  bool `short:"d" long:"debug" description:"Show runtime debug info."`
	Merge  bool `short:"m" long:"merge" description:"Merge all mergeable pull requests."`
	Status bool `short:"s" long:"status" description:"Show status of open pull requests."`
	Update bool `short:"u" long:"update" description:"Update and create pull requests for unmerged commits in the stack."`
}

func main() {
	var opts opts
	_, err := flags.Parse(&opts)
	check(err)

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

	config := &stackediff.Config{
		GitHubRepoOwner: "ejoffe",
		GitHubRepoName:  "apomelo",
	}

	stackediff := stackediff.NewStackedDiff(config)
	if opts.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		stackediff.DebugMode(true)
	}

	if opts.Amend {
		stackediff.AmendCommit(ctx, client)
	} else if opts.Update {
		stackediff.UpdatePullRequests(ctx, client)
	} else if opts.Merge {
		stackediff.MergePullRequests(ctx, client)
		stackediff.UpdatePullRequests(ctx, client)
	} else if opts.Status {
		stackediff.StatusPullRequests(ctx, client)
	} else {
		stackediff.StatusPullRequests(ctx, client)
	}

	stackediff.DebugPrintSummary()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
