package input

import (
	"path/filepath"
	"slices"
	"strings"
)

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
	if len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0]) {
		return path[:2]
	}
	return ""
}

// isASCIILetter reports whether character is an ASCII letter.
func isASCIILetter(character byte) bool {
	return ('a' <= character && character <= 'z') || ('A' <= character && character <= 'Z')
}
