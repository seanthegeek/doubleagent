package parser

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
)

// Beside returns the path of names below the directory that holds the
// artifact's file, and true: where a parser looks for the side files an
// agent keeps next to a transcript or a database. It is Above with levels
// 0, so it is false on the same terms.
func Beside(artifact *input.Artifact, names ...string) (string, bool) {
	return Above(artifact, 0, names...)
}

// Above returns the path of names below the directory levels above the one
// that holds the artifact's file (0 is that directory itself), and true.
// The directory is found from the artifact's path relative to its home
// (Rel) through Home.PathOf, so a file the collector staged at a symlinked
// directory's real place finds its neighbours where the home's own layout
// puts them, and every result lies below the home or a staged location. It
// is false when that would climb past the home, when a name is not one
// plain component (Component), or when the way down passes through a link.
// An artifact without a home, which only a test builds, climbs from its
// disk path instead.
func Above(artifact *input.Artifact, levels int, names ...string) (string, bool) {
	if levels < 0 || levels > strings.Count(artifact.Rel, "/") {
		return "", false
	}
	for _, name := range names {
		if _, ok := Component(name); !ok {
			return "", false
		}
	}
	if artifact.Home == nil {
		directory := filepath.Dir(artifact.DiskPath)
		for range levels {
			directory = filepath.Dir(directory)
		}
		return input.Below(directory, names...)
	}
	directory := path.Dir(artifact.Rel)
	for range levels {
		directory = path.Dir(directory)
	}
	return artifact.Home.PathOf(directory + "/" + strings.Join(names, "/"))
}

// Under returns names joined below directory, a path Beside, Above or
// Home.PathOf gave, and true; it is false when a name is not one plain
// component or when directory itself, or any directory on the way down, is
// a link. A parser extends a path the kit gave only through it, so what it
// opens has been looked at from the home down.
func Under(directory string, names ...string) (string, bool) {
	for _, name := range names {
		if _, ok := Component(name); !ok {
			return "", false
		}
	}
	return input.Below(directory, names...)
}
