package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fsnotify/fsnotify"
	"github.com/matthewmueller/dev/internal/shell"
	"github.com/matthewmueller/text"
)

type Run struct {
	Chdir string
	Clear bool
	Path  string
	Args  []string
}

func (c *CLI) Run(ctx context.Context, in *Run) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	dir, err := c.resolve(in.Chdir)
	if err != nil {
		return err
	}

	if err := c.refreshDeps(w, dir, in.Path); err != nil {
		return err
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	exec := shell.Exec{
		Stderr: c.Stderr,
		Stdout: c.Stdout,
		Stdin:  c.Stdin,
		Env:    c.Env,
		Dir:    dir,
	}

	binName := text.Snake(filepath.Join(dir, in.Path))
	binDir := filepath.Join(cacheDir, "go-dev", "run")
	binPath := filepath.Join(binDir, binName)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	// Build the binary
	if err := exec.Command("go", "build",
		"-mod", "mod",
		"-o", binPath,
		in.Path,
	).Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	// Start the binary
	process, err := exec.Command(binPath, in.Args...).Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	seen := map[string]bool{}

	for {
		select {
		case <-ctx.Done():
			// Canceled context are not errors
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case err := <-w.Errors:
			return err
		case evt := <-w.Events:
			// Sometimes the event name can be empty on Linux during deletes. Ignore
			// those events.
			if evt.Name == "" {
				continue
			}
			// Switch over the operations
			switch op := evt.Op; {
			case op&fsnotify.Rename != 0:
				if err := rename(w, evt.Name); err != nil {
					return fmt.Errorf("dev: unable to rename %q. %w", evt.Name, err)
				}
			case op&fsnotify.Remove != 0:
				if err := remove(w, evt.Name); err != nil {
					return fmt.Errorf("dev: unable to remove %q. %w", evt.Name, err)
				}
			case op&fsnotify.Write != 0:
				stat, err := os.Stat(evt.Name)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				// Ignore the event if it's a duplicate event
				stamp := computeStamp(evt.Name, stat)
				if seen[stamp] {
					continue
				}
				seen[stamp] = true
				// Refresh the watched dependencies list
				if err := c.refreshDeps(w, dir, in.Path); err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				if in.Clear {
					clear()
				}

				// Rebuild the binary
				if err := exec.Command("go", "build",
					"-mod", "mod",
					"-o", binPath,
					in.Path,
				).Run(ctx); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}

				proc, err := process.Restart(ctx)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				process = proc
			}
		}
	}
}

func (c *CLI) refreshDeps(watcher *fsnotify.Watcher, dir, path string) error {
	// Resolve the dependencies
	deps, err := c.deps(dir, &Deps{
		Paths: []string{path},
	})
	if err != nil {
		return err
	}

	// Watch the dependencies
	for _, dep := range deps {
		if err := watcher.Add(dep); err != nil {
			return fmt.Errorf("dev: unable to watch %q. %w", dep, err)
		}
	}
	return nil
}

// Handle renames
func rename(watcher *fsnotify.Watcher, path string) error {
	_, err := os.Stat(path)
	if nil == err {
		return nil
	}
	// If it's a different error, ignore
	if !errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	// Remove the path and emit an update
	watcher.Remove(path)
	return nil
}

// Handle removals
func remove(watcher *fsnotify.Watcher, path string) error {
	watcher.Remove(path)
	return nil
}

// computeStamp uses path, size, mode and modtime to try and ensure this is a
// unique event.
func computeStamp(path string, stat fs.FileInfo) (stamp string) {
	mtime := stat.ModTime().Unix()
	mode := stat.Mode()
	size := stat.Size()
	stamp = path + ":" + strconv.Itoa(int(size)) + ":" + mode.String() + ":" + strconv.Itoa(int(mtime))
	return stamp
}
