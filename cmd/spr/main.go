package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/config/config_parser"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/github/githubclient"
	"github.com/ejoffe/spr/spr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/urfave/cli/v2"
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

func main() {
	gitcmd := realgit.NewGitCmd(config.DefaultConfig())
	//  check that we are inside a git dir
	var output string
	err := gitcmd.Git("status --porcelain", &output)
	if err != nil {
		fmt.Println(output)
		fmt.Println(err)
		os.Exit(2)
	}

	cfg := config_parser.ParseConfig(gitcmd)
	gitcmd = realgit.NewGitCmd(cfg)

	ctx := context.Background()
	client := githubclient.NewGitHubClient(ctx, cfg)
	stackedpr := spr.NewStackedPR(cfg, client, gitcmd)

	detailFlag := &cli.BoolFlag{
		Name:  "detail",
		Value: false,
		Usage: "Show detailed status bits output",
	}

	cli.AppHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.HelpName}} {{if .VisibleFlags}}[global options]{{end}}{{if .Commands}} command [command options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}

GLOBAL OPTIONS:
{{range .VisibleFlags}}{{"\t"}}{{.}}
{{end}}
COMMANDS:
{{range .Commands}}{{if not .HideHelp}}   {{join .Names ","}}{{ "\t"}}{{.Usage}}{{ "\n" }}{{end}}{{end}}
AUTHOR: {{range .Authors}}{{ . }}{{end}}
VERSION: {{.Version}}
`

	app := &cli.App{
		Name:                 "spr",
		Usage:                "Stacked Pull Requests on GitHub",
		HideVersion:          true,
		Version:              fmt.Sprintf("%s : %s : %s\n", version, date, commit[:8]),
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{
				Name:  "Eitan Joffe",
				Email: "eitan@inigolabs.com",
			},
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "detail",
				Value: false,
				Usage: "Show detailed status bits output",
			},
			&cli.BoolFlag{
				Name:  "profile",
				Value: false,
				Usage: "Show runtime profiling info",
			},
			&cli.BoolFlag{
				Name:  "verbose",
				Value: false,
				Usage: "Show verbose logging",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Value: false,
				Usage: "Show runtime debug info",
			},
		},
		Before: func(c *cli.Context) error {
			if c.IsSet("debug") {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
				rake.LoadSources(&cfg, rake.DebugWriter(os.Stdout))
			}
			if c.IsSet("profile") {
				stackedpr.ProfilingEnable()
			}
			if c.IsSet("detail") || cfg.User.StatusBitsHeader {
				stackedpr.DetailEnabled = true
			}
			if c.IsSet("verbose") {
				cfg.User.LogGitCommands = true
				cfg.User.LogGitHubCalls = true
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:    "status",
				Aliases: []string{"s", "st"},
				Usage:   "Show status of open pull requests",
				Action: func(c *cli.Context) error {
					stackedpr.StatusPullRequests(ctx)
					return nil
				},
				Flags: []cli.Flag{
					detailFlag,
				},
			},
			{
				Name:  "sync",
				Usage: "Synchronize local stack with remote",
				Action: func(c *cli.Context) error {
					stackedpr.SyncStack(ctx)
					return nil
				},
			},
			{
				Name:    "update",
				Aliases: []string{"u", "up"},
				Usage:   "Update and create pull requests for updated commits in the stack",
				Action: func(c *cli.Context) error {
					if c.Bool("no-rebase") {
						os.Setenv("SPR_NOREBASE", "true")
					}
					if c.IsSet("count") {
						count := c.Uint("count")
						stackedpr.UpdatePullRequests(ctx, c.StringSlice("reviewer"), &count)
					} else {
						stackedpr.UpdatePullRequests(ctx, c.StringSlice("reviewer"), nil)
					}
					return nil
				},
				Flags: []cli.Flag{
					detailFlag,
					&cli.StringSliceFlag{
						Name:    "reviewer",
						Aliases: []string{"r"},
						Usage:   "Add the specified reviewer to newly created pull requests",
					},
					&cli.UintFlag{
						Name:    "count",
						Aliases: []string{"c"},
						Usage:   "Update a specified number of pull requests from the bottom of the stack",
					},
					&cli.BoolFlag{
						Name:    "no-rebase",
						Aliases: []string{"nr"},
						Usage:   "Disable rebasing",
					},
				},
			},
			{
				Name:  "merge",
				Usage: "Merge all mergeable pull requests",
				Action: func(c *cli.Context) error {
					if c.IsSet("count") {
						count := c.Uint("count")
						stackedpr.MergePullRequests(ctx, &count)
					} else {
						stackedpr.MergePullRequests(ctx, nil)
						stackedpr.UpdatePullRequests(ctx, nil, nil)
					}
					return nil
				},
				Flags: []cli.Flag{
					detailFlag,
					&cli.UintFlag{
						Name:    "count",
						Aliases: []string{"c"},
						Usage:   "Merge a specified number of pull requests from the bottom of the stack",
					},
				},
			},
			{
				Name:  "check",
				Usage: "Run pre merge checks (configured by MergeCheck in repository config)",
				Action: func(c *cli.Context) error {
					stackedpr.RunMergeCheck(ctx)
					return nil
				},
			},
			{
				Name:  "version",
				Usage: "Show version info",
				Action: func(c *cli.Context) error {
					return cli.Exit(c.App.Version, 0)
				},
			},
		},
		After: func(c *cli.Context) error {
			if c.IsSet("profile") {
				stackedpr.ProfilingSummary()
			}
			return nil
		},
	}

	app.Run(os.Args)
}
