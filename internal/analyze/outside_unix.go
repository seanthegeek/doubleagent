//go:build !windows

package analyze

import (
	"path/filepath"
	"strings"
)

// placed returns path as the kernel takes it: as written, a relative path
// from the working directory. Nothing is put in front of it, since the
// working directory can be longer than the system takes in one call, and
// nothing is cleaned, since the kernel steps up from a ".." after a link
// from the link's target; resolve follows the names as the kernel does.
func placed(path string) (string, error) { return path, nil }

// followLink returns the place the link at path leads to, relative to the
// working directory when path and the link's targets are.
// filepath.EvalSymlinks follows it and each link on the way, a ".." in a
// target stepping up from the place reached so far, and fails on a target
// that does not exist (walkSymlinks in path/filepath/symlink.go).
func followLink(path string) (string, error) { return filepath.EvalSymlinks(path) }

// outputParent returns the directory the file system puts path's last
// element in: path up to its last slash, as written, so a ".." after a link
// is resolved as the write resolves it; "/" for a path at the root, "" for
// a path of one element.
func outputParent(path string) string {
	trimmed := strings.TrimRight(path, "/")
	end := strings.LastIndexByte(trimmed, '/')
	switch {
	case trimmed == "" && path != "":
		return "/"
	case end < 0:
		return ""
	}
	if parent := strings.TrimRight(trimmed[:end], "/"); parent != "" {
		return parent
	}
	return "/"
}

// belowGiven returns the path of name in directory, written as directory
// was given with one slash before name and nothing cleaned: filepath.Join
// would remove a ".." the kernel resolves from a link's target, naming
// another directory than the one listed.
func belowGiven(directory, name string) string {
	if strings.HasSuffix(directory, "/") {
		return directory + name
	}
	return directory + "/" + name
}

// outputNameRefusal returns why an output path cannot be used, or "": on
// Linux and macOS a colon is an ordinary character of a name.
func outputNameRefusal(string) string { return "" }
