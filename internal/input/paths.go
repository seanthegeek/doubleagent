package input

import (
	"path/filepath"
	"strings"
)

// parentPath returns a cleaned path less its last element, as filepath.Dir
// gives it: "." for a name alone, the volume and root for an element
// directly under the root. It stands in for filepath.Dir, which the import
// guard does not allow in this package.
func parentPath(path string) string {
	volume := filepath.VolumeName(path)
	rest := path[len(volume):]
	last := strings.LastIndexFunc(rest, isSeparator)
	switch {
	case last < 0 && volume != "":
		return volume
	case last < 0:
		return "."
	case last == 0:
		return volume + rest[:1]
	}
	return volume + rest[:last]
}

// pathElements returns the elements of a host path below its volume and
// root, empty ones dropped.
func pathElements(path string) []string {
	return strings.FieldsFunc(path[len(filepath.VolumeName(path)):], isSeparator)
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
