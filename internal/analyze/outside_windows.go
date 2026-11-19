package analyze

import (
	"io/fs"
	"path/filepath"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// namedEntry returns path's own entry. Windows paths reach the system with
// their . and .. segments removed by name, so a link is not followed here:
// on Windows an input given through a link is compared as the link.
func namedEntry(path string) (fs.FileInfo, error) { return osfile.Lstat(path) }

// parentEntry returns the directory holding path by its text, as Windows
// itself resolves .. by name.
func parentEntry(path string) string { return filepath.Dir(path) }

// absoluteText returns path, joined to working when working is not empty,
// cleaned: Windows removes . and .. segments by name before it resolves a
// path, so the cleaned text names the place opened.
func absoluteText(path, working string) string {
	if working == "" {
		return filepath.Clean(path)
	}
	return filepath.Join(working, path)
}

// outputParent returns the directory holding path's last element by its
// text, as Windows resolves it, or "" for a path of one element.
func outputParent(path string) string {
	if parent := filepath.Dir(path); parent != "." {
		return parent
	}
	return ""
}
