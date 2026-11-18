//go:build !windows

package analyze

import (
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// namedEntry returns what path names: its own entry, or for a link to a
// directory that directory, which an lstat of path/. reaches since only
// the last component is left unfollowed.
func namedEntry(path string) (fs.FileInfo, error) {
	info, err := osfile.Lstat(path)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return info, err
	}
	if followed, err := osfile.Lstat(path + "/."); err == nil {
		return followed, nil
	}
	return info, nil
}

// parentEntry returns the path of the directory holding path's entry as
// the file system has it, through its .. entry, not by cutting the text.
func parentEntry(path string) string { return path + "/.." }
