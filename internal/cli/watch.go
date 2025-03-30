package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/livebud/watcher"
	"github.com/matthewmueller/dev/internal/sh"
)

type Watch struct {
	Ignore  []string
	Clear   bool
	Command string
	Args    []string
}

func (c *CLI) Watch(ctx context.Context, in *Watch) error {
	if in.Clear {
		clear()
	}
	// Run initially
	cmd := sh.Command{
		Stderr: c.Stderr,
		Stdout: c.Stdout,
		Stdin:  c.Stdin,
		Env:    c.Env,
		Dir:    c.Dir,
	}
	if err := cmd.Start(ctx, in.Command, in.Args...); err != nil {
		// Don't exit on errors
		fmt.Fprintln(os.Stderr, err)
	}
	// Watch for changes
	return watcher.Watch(ctx, ".", func(events []watcher.Event) error {
		if len(events) == 0 {
			return nil
		}
		if in.Clear {
			clear()
		}
		if err := cmd.Restart(ctx); err != nil {
			// Don't exit on errors
			fmt.Fprintln(os.Stderr, err)
		}
		return nil
	})
}

func clear() {
	fmt.Fprint(os.Stdout, "\033[H\033[2J")
}
