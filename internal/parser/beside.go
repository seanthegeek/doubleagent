package parser

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Beside returns the path of names below the directory that holds the
// artifact's file, and true: where a parser looks for the side files an
// agent keeps next to a transcript or a database. It is Above with levels
// 0, so it is false on the same terms and leaves the same last element
// unlooked at.
func Beside(artifact *input.Artifact, names ...string) (string, bool) {
	return Above(artifact, 0, names...)
}

// Above returns the path of names below the directory levels above the one
// that holds the artifact's file (0 is that directory itself), and true.
// It goes through Home.PathOf from the artifact's Rel, so a file staged at
// a symlinked directory's real place finds its neighbours where the home's
// layout puts them. It is false when that would climb past the home, when
// a name is not one plain component (Component), or when the way down
// passes through a link; the last element (the directory itself when names
// is empty) is left for the reader, ReadDir or DirectoryAt, none of which
// follows it. An artifact without a home (only a test builds one) climbs
// from its disk path instead.
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

// Under returns names joined below directory, a path Beside, Above,
// Home.PathOf or a Directory gave, and true; it is false when a name is not
// one plain component or when directory itself, or any directory on the way
// down, is a link. The last name is left for the reader, which never
// follows it. Each call looks at every level again, so a parser reading
// many files of one directory takes it once as a Directory instead.
func Under(directory string, names ...string) (string, bool) {
	for _, name := range names {
		if _, ok := Component(name); !ok {
			return "", false
		}
	}
	return input.Below(directory, names...)
}

// Directory is a directory a parser reads several entries of: a path the
// kit built (Beside, Above, Under, Home.PathOf or another Directory's
// Child) whose own entry DirectoryAt found to be a directory, not a link.
// Child joins a name below it without looking at the directory again, so a
// parser looks at a directory once, not once per file it reads there. The
// zero Directory is none: its Child is always false.
type Directory struct {
	path string
}

// DirectoryAt returns the directory at path, a path the kit built, and
// true when path's own entry is a plain directory; false when it is a
// link, anything else or cannot be looked at. On Windows a junction has no
// directory mode and any other reparse point (an entry the file system
// redirects elsewhere) is marked irregular, so neither is taken, as Under
// refuses both. The directories above path were looked at when the kit
// built it.
func DirectoryAt(path string) (Directory, bool) {
	info, err := osfile.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return Directory{}, false
	}
	return Directory{path: path}, true
}

// Path returns the directory's path, "" for the zero Directory.
func (d Directory) Path() string { return d.path }

// Child returns name joined below the directory and true. It is false when
// name is not one plain component (Component, and on Windows no colon, as
// Under refuses it) or the Directory is the zero one. The directory is not
// looked at again, and the name's own entry is left for the reader, which
// never follows it.
func (d Directory) Child(name string) (string, bool) {
	if _, ok := Component(name); !ok || !platformName(name) || d.path == "" {
		return "", false
	}
	return filepath.Join(d.path, name), true
}
