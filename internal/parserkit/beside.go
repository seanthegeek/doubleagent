package parserkit

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Above returns the path of names below the directory levels above the one
// that holds the artifact's file (0 is that directory itself), and true;
// false when it would climb past the home, a name is not one component, or
// the way down passes through a link. It goes through Home.PathOf from the
// artifact's Rel, so a file staged at a symlinked directory's real place
// finds its neighbours where the home's layout puts them; an artifact
// without a home (only a test builds one) climbs from its disk path.
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
		return artifact.Home.Below(directory, names...)
	}
	directory := path.Dir(artifact.Rel)
	for range levels {
		directory = path.Dir(directory)
	}
	return artifact.Home.PathOf(directory + "/" + strings.Join(names, "/"))
}

// Beside returns the path of names below the directory that holds the
// artifact's file, and true: where an agent keeps side files next to a
// transcript or a database. It is Above with levels 0.
func Beside(artifact *input.Artifact, names ...string) (string, bool) {
	return Above(artifact, 0, names...)
}

// Under returns names joined below directory, a path the kit built for the
// artifact, and true; false when a name is not one component or directory
// or any directory on the way down is a link. A name that does not exist is
// found in another ASCII case from the listings the artifact's home keeps
// (input.Home.Listing). Each call looks at every level again, so a parser
// reading many files of one directory takes it once as a Directory instead.
func Under(artifact *input.Artifact, directory string, names ...string) (string, bool) {
	for _, name := range names {
		if _, ok := Component(name); !ok {
			return "", false
		}
	}
	return artifact.Home.Below(directory, names...)
}

// Sidecar returns the path of a database's sidecar file, the database's
// path, a path the kit built for the artifact, with suffix appended: one of
// "-wal", "-shm" and "-journal", the files SQLite keeps beside a database
// under its name. It is false for any other suffix. A sidecar absent as
// built is found in another ASCII case from the listings the artifact's
// home keeps.
func Sidecar(artifact *input.Artifact, path, suffix string) (string, bool) {
	switch suffix {
	case "-wal", "-shm", "-journal":
		if path == "" {
			return "", false
		}
		if _, err := osfile.Lstat(path + suffix); !errors.Is(err, fs.ErrNotExist) {
			return path + suffix, true
		}
		directory := filepath.Dir(path)
		return filepath.Join(directory, artifact.Home.Listing(directory).Spelling(filepath.Base(path)+suffix)), true
	}
	return "", false
}

// Directory is a directory a parser reads several entries of, which
// DirectoryAt found to be a directory, not a link, so it is looked at once,
// not once per file read there. The zero Directory is none: its Child is
// always false.
type Directory struct {
	path string
	// home is the home of the artifact the directory was found for, whose
	// kept listings Child answers from; nil for an artifact without one.
	home *input.Home
}

// DirectoryAt returns the directory at path, a path the kit built for the
// artifact, and true when path's own entry is a plain directory; false when
// it is a link, anything else or cannot be looked at. On Windows a junction
// has no directory mode and any other reparse point (an entry the file
// system redirects elsewhere) is marked irregular, so neither is taken, as
// Under refuses both.
func DirectoryAt(artifact *input.Artifact, path string) (Directory, bool) {
	info, err := osfile.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return Directory{}, false
	}
	return Directory{path: path, home: artifact.Home}, true
}

// Path returns the directory's path, "" for the zero Directory.
func (d Directory) Path() string { return d.path }

// Child returns name joined below the directory and true; false when name
// is not one component or the Directory is the zero one. The name is taken
// as the directory's listing spells it when the listing holds no entry of
// name's bytes and one equal under ASCII fold (osfile.Spellings), else as
// built; a listing that fails gives it as built. The listing is the one the
// home keeps for the run (input.Home.Listing), so a directory is listed once
// however many names, artifacts and Directories ask of it, and no name is
// looked at.
func (d Directory) Child(name string) (string, bool) {
	if _, ok := Component(name); !ok || d.path == "" {
		return "", false
	}
	return filepath.Join(d.path, d.home.Listing(d.path).Spelling(name)), true
}
