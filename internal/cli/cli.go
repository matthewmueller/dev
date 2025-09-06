package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/livebud/cli"
	"github.com/matthewmueller/dev/internal/gitignore"
)

func New() *CLI {
	return &CLI{
		Dir:    ".",
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

func (c *CLI) resolve(dir string) (string, error) {
	if filepath.IsAbs(dir) {
		return filepath.EvalSymlinks(dir)
	}
	absDir, err := filepath.Abs(c.Dir)
	if err != nil {
		return "", err
	}
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(absDir, dir), nil
}

func (c *CLI) gitIgnore(dir string, noGitIgnore bool) (func(path string) (ignore bool), error) {
	if noGitIgnore {
		return func(path string) bool {
			return false
		}, nil
	}
	gitIgnore, err := gitignore.From(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	return gitIgnore, nil
}

func (c *CLI) Parse(ctx context.Context, args ...string) error {
	cli := cli.New("dev", "personal dev tooling")

	{ // serve [flags] [dir]
		in := new(Serve)
		cmd := cli.Command("serve", "serve a directory")
		cmd.Flag("listen", "address to listen on").String(&in.Listen).Default(":3000")
		cmd.Flag("live", "enable live reloading").Bool(&in.Live).Default(true)
		cmd.Flag("open", "open browser").Bool(&in.Browser).Default(true)
		cmd.Arg("dir", "directory to serve").String(&in.Dir).Default(".")
		cmd.Flag("include", "include files matching pattern").Short('I').Strings(&in.Includes).Default()
		cmd.Flag("exclude", "exclude files matching pattern").Short('E').Strings(&in.Excludes).Default()
		cmd.Flag("no-gitignore", "ignore .gitignore").Bool(&in.NoGitIgnore).Default(false)
		cmd.Run(func(ctx context.Context) error { return c.Serve(ctx, in) })
	}

	{ // watch [flags] [dir]
		in := new(Watch)
		cmd := cli.Command("watch", "watch a directory")
		cmd.Flag("clear", "clear screen every change").Bool(&in.Clear).Default(false)
		cmd.Arg("command", "command to run").String(&in.Command)
		cmd.Args("args", "command arguments").Strings(&in.Args).Default()
		cmd.Flag("include", "include files matching pattern").Short('I').Strings(&in.Includes).Default()
		cmd.Flag("exclude", "exclude files matching pattern").Short('E').Strings(&in.Excludes).Default()
		cmd.Flag("no-gitignore", "ignore .gitignore").Bool(&in.NoGitIgnore).Default(false)
		cmd.Run(func(ctx context.Context) error { return c.Watch(ctx, in) })
	}

	{ // run [flags] [path]
		in := new(Run)
		cmd := cli.Command("run", "run a Go file")
		cmd.Flag("clear", "clear screen every change").Bool(&in.Clear).Default(false)
		cmd.Flag("chdir", "change directory").Short('C').String(&in.Chdir).Default(".")
		cmd.Arg("path", "path to Go file").String(&in.Path)
		cmd.Args("args", "command arguments").Strings(&in.Args).Default()
		cmd.Run(func(ctx context.Context) error { return c.Run(ctx, in) })
	}

	{ // txtar
		cmd := cli.Command("txtar", "txtar tools").Advanced()

		{ // txtar pack <dir>
			in := new(TxtarPack)
			cmd := cmd.Command("pack", "pack a directory to stdout")
			cmd.Flag("include", "include files matching pattern").Short('I').Strings(&in.Includes).Default()
			cmd.Flag("exclude", "exclude files matching pattern").Short('E').Strings(&in.Excludes).Default()
			cmd.Flag("no-gitignore", "ignore .gitignore").Bool(&in.NoGitIgnore).Default(false)
			cmd.Arg("dir", "directory to pack").String(&in.Dir).Default(".")
			cmd.Run(func(ctx context.Context) error { return c.TxtarPack(ctx, in) })
		}

		{ // txtar unpack
			in := new(TxtarUnpack)
			cmd := cmd.Command("unpack", "unpack a txtar file to a directory")
			cmd.Arg("path", "input txtar file").String(&in.Path)
			cmd.Arg("dir", "output directory").String(&in.Dir).Default(".")
			cmd.Flag("force", "overwrite existing files").Bool(&in.Force).Default(false)
			cmd.Flag("include", "include files matching pattern").Short('I').Strings(&in.Includes).Default()
			cmd.Flag("exclude", "exclude files matching pattern").Short('E').Strings(&in.Excludes).Default()
			cmd.Flag("no-gitignore", "ignore .gitignore").Bool(&in.NoGitIgnore).Default(false)
			cmd.Run(func(ctx context.Context) error { return c.TxtarUnpack(ctx, in) })
		}
	}

	{ // deps
		in := new(Deps)
		cmd := cli.Command("deps", "list dependencies").Advanced()
		cmd.Args("path", "path to file").Strings(&in.Paths).Default()
		cmd.Flag("tests", "include test dependencies").Bool(&in.IncludeTests).Default(false)
		cmd.Flag("modules", "include installed modules").Bool(&in.IncludeModules).Default(false)
		cmd.Flag("stdlib", "include standard library").Bool(&in.IncludeStdlib).Default(false)
		cmd.Run(func(ctx context.Context) error { return c.Deps(ctx, in) })
	}

	{ // version
		cmd := cli.Command("version", "print the version")
		cmd.Run(func(ctx context.Context) error {
			fmt.Fprintln(c.Stdout, "v"+version)
			return nil
		})
	}

	return cli.Parse(ctx, os.Args[1:]...)
}

func clear() {
	fmt.Fprint(os.Stdout, "\033[H\033[2J")
}
