package gitignore

import (
	"errors"
	"io/fs"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

var alwaysIgnore = []string{
	".git",
}

var defaultIgnore = FromLines(
	".git",
	"node_modules",
	".DS_Store",
)

// Ignore checks if a path should be ignored against the default ignores
func Ignore(path string) bool {
	return defaultIgnore(path)
}

// FromFS reads a .gitignore file from the filesystem
func From(fsys fs.FS) (ignore func(path string) bool, err error) {
	code, err := fs.ReadFile(fsys, ".gitignore")
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return defaultIgnore, nil
	}
	lines := strings.Split(string(code), "\n")
	lines = append(lines, alwaysIgnore...)
	ignore = FromLines(lines...)
	return ignore, nil
}

// Compile a list of gitignore lines
func FromLines(lines ...string) (ignore func(path string) bool) {
	ignorer := gitignore.CompileIgnoreLines(lines...)
	return ignorer.MatchesPath
}
