package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
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
	gitcmd := realgit.NewGitCmd(&config.Config{})
	//  check that we are inside a git dir
	var output string
	err := gitcmd.Git("status --porcelain", &output)
	if err != nil {
		fmt.Println(output)
		fmt.Println(err)
		os.Exit(2)
	}

	// parse configuration
	cfg := config.Config{}
	configfilepath := config.ConfigFilePath(gitcmd)
	rake.LoadSources(&cfg,
		rake.DefaultSource(),
		config.GitHubRemoteSource(&cfg, gitcmd),
		rake.YamlFileSource(configfilepath),
		rake.YamlFileWriter(configfilepath),
	)

	if cfg.GitHubRepoOwner == "" {
		fmt.Println("unable to auto configure repository owner - must be set manually in .spr.yml")
		os.Exit(3)
	}

	if cfg.GitHubRepoName == "" {
		fmt.Println("unable to auto configure repository name - must be set manually in .spr.yml")
		os.Exit(4)
	}

	gitcmd = realgit.NewGitCmd(&cfg)

	ctx := context.Background()
	client := githubclient.NewGitHubClient(ctx, &cfg)
	stackedpr := spr.NewStackedPR(&cfg, client, gitcmd, os.Stdout)

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
				Email: "eitan@apomelo.com",
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
			if c.IsSet("detail") || cfg.StatusBitsHeader {
				stackedpr.DetailEnabled = true
			}
			if c.IsSet("verbose") {
				cfg.LogGitCommands = true
				cfg.LogGitHubCalls = true
			}
			return nil
		},
		Action: func(c *cli.Context) error {
			stackedpr.StatusPullRequests(ctx)
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:  "status",
				Usage: "Show status of open pull requests",
				Action: func(c *cli.Context) error {
					stackedpr.StatusPullRequests(ctx)
					return nil
				},
				Flags: []cli.Flag{
					detailFlag,
				},
			},
			{
				Name:  "update",
				Usage: "Update and create pull requests for updated commits in the stack",
				Action: func(c *cli.Context) error {
					stackedpr.UpdatePullRequests(ctx)
					return nil
				},
				Flags: []cli.Flag{
					detailFlag,
				},
			},
			{
				Name:  "merge",
				Usage: "Merge all mergeable pull requests",
				Action: func(c *cli.Context) error {
					stackedpr.MergePullRequests(ctx)
					stackedpr.UpdatePullRequests(ctx)
					return nil
				},
				Flags: []cli.Flag{
					detailFlag,
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
