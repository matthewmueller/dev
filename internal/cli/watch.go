package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/livebud/watcher"
	"github.com/matthewmueller/dev/internal/matcher"
	"github.com/matthewmueller/dev/internal/sh"
)

type Watch struct {
	Clear       bool
	Command     string
	Args        []string
	NoGitIgnore bool
	Includes    []string
	Excludes    []string
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
	command, args, err := formatCommand(in.Command, in.Args)
	if err != nil {
		return fmt.Errorf("failed to format command %s %+v: %w", in.Command, in.Args, err)
	}
	gitIgnore, err := c.gitIgnore(c.Dir, in.NoGitIgnore)
	if err != nil {
		return err
	}
	match, err := matcher.Compile(in.Includes, in.Excludes, gitIgnore)
	if err != nil {
		return fmt.Errorf("failed to create matcher: %w", err)
	}
	if err := cmd.Start(ctx, command, args...); err != nil {
		// Don't exit on errors
		fmt.Fprintln(os.Stderr, err)
	}
	// Watch for changes
	return watcher.Watch(ctx, ".", func(events []watcher.Event) error {
		if len(events) == 0 {
			return nil
		}
		if len(filterEvents(events, match)) == 0 {
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

func formatCommand(cmd string, args []string) (string, []string, error) {
	if len(args) > 0 {
		return cmd, args, nil
	}
	if isMultipleCommands(cmd) {
		return "sh", []string{
			"-c",
			cmd,
		}, nil
	}
	words, err := shellquote.Split(cmd)
	if err != nil || len(words) == 0 {
		return "", nil, err
	}
	if len(words) == 1 {
		return words[0], nil, nil
	}
	return words[0], words[1:], nil
}

func isMultipleCommands(cmd string) bool {
	ops := []string{"&&", "||", ";", "|", "`", "$(", ")", "{", "}", ">", "<", ">>", "2>", "1>", "2>&1", "1>&2"}
	for _, op := range ops {
		if strings.Contains(cmd, op) {
			return true
		}
	}
	return false
}

func filterEvents(events []watcher.Event, match func(path string) bool) (filtered []watcher.Event) {
	for _, event := range events {
		if match(event.Path) {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
