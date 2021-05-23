package main

import (
	"context"

	"github.com/ejoffe/spr/spr"
)

func main() {
	ctx := context.Background()
	spr.AmendCommit(ctx)
}
