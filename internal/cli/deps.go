package cli

import (
	"context"
	"fmt"
	"go/build"
	"path/filepath"

	"github.com/livebud/mod"
)

type Deps struct {
	Paths          []string
	IncludeTests   bool
	IncludeModules bool
	IncludeStdlib  bool
}

type depSet struct {
	paths []string
	seen  map[string]bool
}

func (d *depSet) Add(path string) {
	if d.seen[path] {
		return
	}
	d.seen[path] = true
	d.paths = append(d.paths, path)
}

func (d *depSet) List() []string {
	return d.paths
}

func (c *CLI) Deps(ctx context.Context, in *Deps) error {
	if len(in.Paths) == 0 {
		return nil
	}
	dir, err := c.resolve(c.Dir)
	if err != nil {
		return err
	}
	deps, err := c.deps(dir, in)
	if err != nil {
		return err
	}
	for _, dep := range deps {
		rel, err := filepath.Rel(dir, dep)
		if err != nil {
			return fmt.Errorf("dev: unable to get relative path %q. %w", dep, err)
		}
		fmt.Fprintln(c.Stdout, rel)
	}
	return nil
}

func (c *CLI) deps(dir string, in *Deps) ([]string, error) {
	module, err := mod.Find(dir)
	if err != nil {
		return nil, fmt.Errorf("dev: unable to find module %q. %w", dir, err)
	}
	set := &depSet{
		seen: make(map[string]bool),
	}
	seenDir := map[string]bool{}
	// TODO: concurrency
	for _, path := range in.Paths {
		absPath, err := c.resolve(path)
		if err != nil {
			return nil, fmt.Errorf("dev: unable to resolve path %q. %w", path, err)
		}
		if err := c.depsOf(set, seenDir, in, module, filepath.Dir(absPath)); err != nil {
			return nil, fmt.Errorf("dev: unable to walk path %q. %w", path, err)
		}
	}
	return set.List(), nil
}

func (c *CLI) depsOf(set *depSet, seenDir map[string]bool, in *Deps, module *mod.Module, dir string) error {
	pkg, err := build.Default.ImportDir(dir, build.ImportMode(0))
	if err != nil {
		return fmt.Errorf("deps: unable to import directory %q. %w", dir, err)
	}
	for _, path := range pkg.GoFiles {
		set.Add(filepath.Join(dir, path))
	}
	if in.IncludeTests {
		for _, path := range pkg.TestGoFiles {
			set.Add(filepath.Join(dir, path))
		}
		for _, path := range pkg.XTestGoFiles {
			set.Add(filepath.Join(dir, path))
		}
	}

	seenDir[dir] = true

	for _, importPath := range pkg.Imports {
		if !module.Contains(importPath) && !in.IncludeModules {
			continue
		} else if mod.InStdlib(importPath) && !in.IncludeStdlib {
			continue
		}
		importDir, err := module.ResolveDir(importPath)
		if err != nil {
			return fmt.Errorf("deps: unable to resolve directory from import path %q. %w", importPath, err)
		}
		if seenDir[importDir] {
			continue
		}
		if err := c.depsOf(set, seenDir, in, module, importDir); err != nil {
			return err
		}
	}

	return nil
}
