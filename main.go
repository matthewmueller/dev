package main

import (
	"context"
	"os"

	"github.com/matthewmueller/dev/internal/cli"
	"github.com/matthewmueller/logs"
)

func main() {
	cli := cli.New()
	ctx := context.Background()
	if err := cli.Parse(ctx, os.Args[1:]...); err != nil {
		logs.Error(err.Error())
	}
}
