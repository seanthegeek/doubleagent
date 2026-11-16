//go:build darwin || linux

package stage

import (
	"io/fs"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// spelledBelow reports whether every component of path below base is an
// entry its directory lists under exactly that name, so a directory that
// folds case has not resolved one of them to an entry spelled otherwise.
func spelledBelow(base, path string) bool {
	rest, found := strings.CutPrefix(path, base+"/")
	if !found {
		return true
	}
	directory := base
	for _, component := range strings.Split(rest, "/") {
		entries, err := osfile.ReadDir(directory)
		if err != nil || !slices.ContainsFunc(entries, func(entry fs.DirEntry) bool {
			return entry.Name() == component
		}) {
			return false
		}
		directory += "/" + component
	}
	return true
}
