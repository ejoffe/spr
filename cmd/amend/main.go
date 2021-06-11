package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ejoffe/spr/git/realgit"
	"github.com/ejoffe/spr/spr"
	"github.com/jessevdk/go-flags"
)

var (
	version = "dev"
	commit  = "dversion"
	date    = "unknown"
)

// command line opts
type opts struct {
	Version bool `short:"v" long:"version" description:"Show version info."`
}

func main() {
	var opts opts
	_, err := flags.Parse(&opts)
	check(err)

	if opts.Version {
		fmt.Printf("amend version : %s : %s : %s\n", version, date, commit[:8])
		os.Exit(0)
	}

	ctx := context.Background()
	sd := spr.NewStackedPR(nil, nil, realgit.Cmd, os.Stdout, false)
	sd.AmendCommit(ctx)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
