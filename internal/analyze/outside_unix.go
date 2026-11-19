//go:build !windows

package analyze

import (
	"os"
	"path/filepath"
	"strings"
)

// absolute returns path made absolute by putting the working directory in
// front of a relative one, without cleaning the text: the kernel steps up
// from a ".." after a link from the link's target, so cleaning the text
// would name another place than the one the write opens. resolve follows
// the names as the kernel does.
func absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	working, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return working + "/" + path, nil
}

// followLink returns the place the link at path, already absolute, leads
// to. filepath.EvalSymlinks follows it and each link on the way, a ".." in
// a target stepping up from the place reached so far, and fails on a
// target that does not exist (walkSymlinks in path/filepath/symlink.go).
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
