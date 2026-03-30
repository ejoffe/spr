package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ejoffe/rake"
	"github.com/ejoffe/spr/config"
	"github.com/ejoffe/spr/config/config_parser"
	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/github/githubclient"
	"github.com/ejoffe/spr/spr"
	"github.com/ejoffe/spr/vcs"
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

// handleEditSequence is an internal command used as a git sequence editor.
// It rewrites 'pick <hash>' to 'edit <hash>' for a target commit in the rebase todo file.
// Usage: spr _edit-sequence <commit-hash-prefix> <todo-file>
func handleEditSequence() {
	if len(os.Args) < 4 || os.Args[1] != "_edit-sequence" {
		return
	}
	hashPrefix := os.Args[2]
	todoFile := os.Args[3]

	data, err := os.ReadFile(todoFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading todo file: %s\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var lines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "pick "+hashPrefix) {
			line = strings.Replace(line, "pick ", "edit ", 1)
		}
		lines = append(lines, line)
	}

	err = os.WriteFile(todoFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error writing todo file: %s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	// Handle internal _edit-sequence command before any git/config initialization.
	// This is invoked by git as a sequence editor during 'spr edit'.
	handleEditSequence()

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

	err = config_parser.CheckConfig(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	gitcmd = realgit.NewGitCmd(cfg)

	// Check for --no-jj flag or SPR_NOJJ env var before creating VCS operations.
	// This must happen before app.Run() since vcsOps is created here.
	for _, arg := range os.Args[1:] {
		if arg == "--no-jj" {
			cfg.User.NoJJ = true
		}
	}
	if os.Getenv("SPR_NOJJ") == "true" {
		cfg.User.NoJJ = true
	}

	ctx := context.Background()
	client := githubclient.NewGitHubClient(ctx, cfg)
	vcsOps := vcs.NewVCSOperations(cfg, gitcmd)
	stackedpr := spr.NewStackedPR(cfg, client, gitcmd, vcsOps)

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
VERSION: fork of {{.Version}}
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
			&cli.BoolFlag{
				Name:    "no-jj",
				Value:   false,
				Usage:   "Disable jj (Jujutsu) mode even in jj-colocated repos",
				EnvVars: []string{"SPR_NOJJ"},
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
			client.MaybeStar(ctx, cfg)
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
				Before: func(c *cli.Context) error {
					// only override whatever was set in yaml if flag is explicitly present
					if c.IsSet("no-rebase") {
						cfg.User.NoRebase = c.Bool("no-rebase")
					}
					return nil
				},
				Action: func(c *cli.Context) error {
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
						// this env var is needed as previous versions used the env var itself to pass intent to logic
						// layer ops so it is likely relied on as a feature by users at this point
						EnvVars: []string{"SPR_NOREBASE"},
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
			Name:    "amend",
			Aliases: []string{"a"},
			Usage:   "Amend a commit in the stack",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "Run spr update after amend",
				},
			},
			Action: func(c *cli.Context) error {
				stackedpr.AmendCommit(ctx)
				if c.Bool("update") {
					stackedpr.UpdatePullRequests(ctx, nil, nil)
				}
				return nil
			},
		},
		{
			Name:    "edit",
			Aliases: []string{"e"},
			Usage:   "Edit a commit in the stack",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "done",
					Aliases: []string{"d"},
					Usage:   "Finish editing and restore the stack",
				},
				&cli.BoolFlag{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "Run spr update after finishing edit (use with --done)",
				},
				&cli.BoolFlag{
					Name:  "abort",
					Usage: "Abort the current edit session",
				},
			},
			Action: func(c *cli.Context) error {
				if c.Bool("abort") {
					stackedpr.EditCommitAbort(ctx)
				} else if c.Bool("done") {
					stackedpr.EditCommitDone(ctx, c.Bool("update"))
				} else {
					stackedpr.EditCommit(ctx)
				}
				return nil
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
