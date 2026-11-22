//go:build unix

package imagetest

import (
	"io/fs"
	"os"
)

// kindOf returns the kind of the entry at path, whose Lstat is info.
func kindOf(path string, info fs.FileInfo) (Kind, error) {
	return kindOfMode(path, info.Mode())
}

// modeOf returns the permission bits a manifest records for the entry: none
// for a link, whose own mode the builder cannot set on Linux.
func modeOf(kind Kind, info fs.FileInfo) fs.FileMode {
	if !hasMode(kind) {
		return 0
	}
	return info.Mode().Perm()
}

// linkTarget returns the target of the link at path, as stored.
func linkTarget(path string, _ Kind) (string, error) {
	return os.Readlink(path)
}
