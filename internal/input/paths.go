package input

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// samePath reports whether two cleaned paths name one place, compared as
// this platform compares path keys (syspath.Fold).
func samePath(first, second string) bool { return syspath.Fold(first) == syspath.Fold(second) }

// trimRightSlash removes trailing slashes.
func trimRightSlash(text string) string { return strings.TrimRight(text, "/") }

// pathElements returns the elements of a host path below its volume and
// root, empty ones dropped.
func pathElements(path string) []string {
	below := filepath.ToSlash(path[len(filepath.VolumeName(path)):])
	return slices.DeleteFunc(strings.Split(below, "/"), func(element string) bool { return element == "" })
}

// joinPath returns base with elements joined to it by filepath.Join.
func joinPath(base string, elements ...string) string {
	return filepath.Join(append([]string{base}, elements...)...)
}

// windowsDrive returns the `X:` drive that starts path, or "".
func windowsDrive(path string) string {
	if len(path) >= 2 && path[1] == ':' && evidencepath.IsASCIILetter(path[0]) {
		return path[:2]
	}
	return ""
}
