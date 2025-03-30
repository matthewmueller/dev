package cli

import (
	"context"
	"io"
	"os"

	"github.com/livebud/cli"
)

func New() *CLI {
	return &CLI{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		Env:    os.Environ(),
	}
}

type CLI struct {
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Env    []string
}

func (c *CLI) Parse(ctx context.Context, args ...string) error {
	cli := cli.New("dev", "personal dev tooling")

	{ // serve [flags] [dir]
		in := new(Serve)
		cli := cli.Command("serve", "serve a directory")
		cli.Flag("listen", "address to listen on").String(&in.Listen).Default(":3000")
		cli.Flag("live", "enable live reloading").Bool(&in.Live).Default(true)
		cli.Flag("open", "open browser").Bool(&in.Browser).Default(true)
		cli.Arg("dir").String(&in.Dir).Default(".")
		cli.Run(func(ctx context.Context) error { return c.Serve(ctx, in) })
	}

	{ // watch [flags] [dir]
		in := new(Watch)
		cmd := cli.Command("watch", "watch a directory")
		cmd.Flag("ignore", "ignore files matching pattern").Strings(&in.Ignore).Default()
		cmd.Flag("clear", "clear screen every change").Bool(&in.Clear).Default(true)
		cmd.Arg("command").String(&in.Command)
		cmd.Args("args").Strings(&in.Args).Default()
		cmd.Run(func(ctx context.Context) error { return c.Watch(ctx, in) })
	}

	return cli.Parse(ctx, os.Args[1:]...)
}
