package gitignore_test

import (
	"testing"
	"testing/fstest"

	"github.com/matryer/is"
	"github.com/matthewmueller/dev/internal/gitignore"
)

func TestBudRoot(t *testing.T) {
	is := is.New(t)
	ignore, err := gitignore.From(fstest.MapFS{
		".gitignore": &fstest.MapFile{Data: []byte(`/bud`)},
	})
	is.NoErr(err)
	is.True(ignore("bud/internal/web/web.go"))
	is.True(!ignore("main.go"))
}

func TestGitDir(t *testing.T) {
	is := is.New(t)
	ignore, err := gitignore.From(fstest.MapFS{
		".gitignore": &fstest.MapFile{Data: []byte(``)},
	})
	is.NoErr(err)
	is.True(ignore(".git"))
	is.True(ignore(".git/objects"))
}

func TestInclude(t *testing.T) {
	is := is.New(t)
	ignorer := gitignore.FromLines("/dist")
	ignore := func(path string) bool {
		return !ignorer(path)
	}
	is.True(!ignore("dist"))
	is.True(ignore("node_modules"))
	is.True(ignore("node_modules/netlify-cli/node_modules/unstorage/dist/shared/unstorage.7746300e.cjs"))
}
