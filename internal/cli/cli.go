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
		cmd := cli.Command("serve", "serve a directory")
		cmd.Flag("include", "include files matching pattern").Short('I').Strings(&in.Includes).Default()
		cmd.Flag("exclude", "exclude files matching pattern").Short('E').Strings(&in.Excludes).Default()
		cmd.Flag("listen", "address to listen on").String(&in.Listen).Default(":3000")
		cmd.Flag("live", "enable live reloading").Bool(&in.Live).Default(true)
		cmd.Flag("open", "open browser").Bool(&in.Browser).Default(true)
		cmd.Arg("dir").String(&in.Dir).Default(".")
		cmd.Run(func(ctx context.Context) error { return c.Serve(ctx, in) })
	}

	{ // watch [flags] [dir]
		in := new(Watch)
		cmd := cli.Command("watch", "watch a directory")
		cmd.Flag("include", "include files matching pattern").Short('I').Strings(&in.Includes).Default()
		cmd.Flag("exclude", "exclude files matching pattern").Short('E').Strings(&in.Excludes).Default()
		cmd.Flag("clear", "clear screen every change").Bool(&in.Clear).Default(false)
		cmd.Arg("command").String(&in.Command)
		cmd.Args("args").Strings(&in.Args).Default()
		cmd.Run(func(ctx context.Context) error { return c.Watch(ctx, in) })
	}

	return cli.Parse(ctx, os.Args[1:]...)
}
