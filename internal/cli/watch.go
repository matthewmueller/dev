package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/livebud/watcher"
	"github.com/matthewmueller/dev/internal/sh"
	"github.com/matthewmueller/glob"
)

type Watch struct {
	Includes []string
	Excludes []string
	Clear    bool
	Command  string
	Args     []string
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
	includes, err := includer(in.Includes...)
	if err != nil {
		return fmt.Errorf("failed to parse include patterns: %w", err)
	}
	excludes, err := excluder(in.Excludes...)
	if err != nil {
		return fmt.Errorf("failed to parse exclude patterns: %w", err)
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
		if !match(events, includes, excludes) {
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

func match(events []watcher.Event, includes func(path string) bool, excludes func(path string) bool) bool {
	// Check if any of the events match the include/exclude rules
	for _, event := range events {
		if !excludes(event.Path) {
			if includes(event.Path) {
				return true // At least one event matches the include rules
			}
		}
	}
	return false // No events matched the include rules
}

func isGlob(pattern string) bool {
	return glob.Base(pattern) != pattern
}

func compilePattern(pattern string) (func(path string) bool, error) {
	if isGlob(pattern) {
		matcher, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}
		return matcher.Match, nil
	}
	return func(path string) bool {
		return strings.Contains(path, pattern)
	}, nil
}

func includer(includes ...string) (func(path string) bool, error) {
	matchers := make([]func(path string) bool, len(includes))
	for i, include := range includes {
		matcher, err := compilePattern(include)
		if err != nil {
			return nil, err
		}
		matchers[i] = matcher
	}
	return func(path string) bool {
		if len(matchers) == 0 {
			return true
		}
		// Include if any of the matchers match
		for _, matcher := range matchers {
			if matcher(path) {
				return true
			}
		}
		return false
	}, nil
}

func excluder(excludes ...string) (func(path string) bool, error) {
	matchers := make([]func(path string) bool, len(excludes))
	for i, exclude := range excludes {
		matcher, err := compilePattern(exclude)
		if err != nil {
			return nil, err
		}
		matchers[i] = matcher
	}
	return func(path string) bool {
		if len(matchers) == 0 {
			return false
		}
		// Exclude if any of the matchers match
		for _, matcher := range matchers {
			if matcher(path) {
				return true
			}
		}
		return false
	}, nil
}
