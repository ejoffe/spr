package main

import (
	"os"

	"github.com/ejoffe/spr/hook"
)

func main() {
	hook.CommitHook(os.Args[1])
}
