package input

import (
	"path/filepath"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// memberSeparators returns an archive member name with this platform's
// path separators as slashes: on Windows a backslash is one too.
func memberSeparators(name string) string { return strings.ReplaceAll(name, `\`, "/") }

// memberNameAllowed reports whether this platform can hold a name as one
// component below a directory without it naming another place. On Windows
// a backslash separates components, and a colon names a drive or an
// alternate data stream of a file, so a name holding either is refused.
func memberNameAllowed(name string) bool { return !strings.ContainsAny(name, `:\`) }

// absolute returns the path Windows reads for path. A path with a drive and
// a root, or a UNC path, is what osfile opens in syspath.Extended's form:
// "." and ".." removed by name and a last name ending in a dot or a space
// kept; a bare drive (D:) is its root, and the \\?\ and \\.\ forms are kept
// as given. Windows resolves any other path (relative, \case, D:case) with
// GetFullPathNameW (filepath.Abs), which drops a final dot or space.
func absolute(path string) (string, error) {
	switch {
	case !syspath.IsAbs(path):
		return filepath.Abs(path)
	case syspath.Extended(path) == path:
		return path, nil
	}
	return syspath.ToNative(syspath.Opened(path)), nil
}
