// Package evidencepath reads a path as the evidence's machine wrote it: a
// Windows path recorded on a Windows host and analysed on a Mac, or a POSIX
// path analysed on Windows. path/filepath follows the rules of the host the
// binary runs on, so it cannot read these; path reads slash forms only. The
// few rules here (a drive letter, either slash as a Windows separator) are
// the ones both the analysis and the project discovery of `collect` need.
package evidencepath

import "strings"

// IsDrivePath reports whether path starts with a drive letter, a colon and
// a separator of either kind (C:\ or c:/).
func IsDrivePath(path string) bool {
	return len(path) >= 3 && IsASCIILetter(path[0]) && path[1] == ':' &&
		(path[2] == '\\' || path[2] == '/')
}

// IsASCIILetter reports whether char is an ASCII letter.
func IsASCIILetter(char byte) bool {
	return char|0x20 >= 'a' && char|0x20 <= 'z'
}

// Elements returns the non-empty elements of path: split at backslashes and
// slashes for a Windows path, at slashes only for a POSIX one, where a
// backslash is a legal character of a file name.
func Elements(path string, windows bool) []string {
	return strings.FieldsFunc(path, func(char rune) bool {
		return char == '/' || windows && char == '\\'
	})
}
