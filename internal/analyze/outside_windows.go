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
