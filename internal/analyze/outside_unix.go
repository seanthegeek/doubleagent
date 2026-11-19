//go:build !windows

package analyze

import (
	"io/fs"
	"strings"

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

// absoluteText returns path, joined to working when working is not empty,
// as text: Unix resolves a ".." after a link through the link's target, so
// cleaning the text would name another place than the one opened.
func absoluteText(path, working string) string {
	if working == "" {
		return path
	}
	return working + "/" + path
}

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
