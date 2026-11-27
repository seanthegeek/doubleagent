package evidencepath

import "strings"

// IsASCIILetter reports whether char is an ASCII letter.
func IsASCIILetter(char byte) bool {
	return char|0x20 >= 'a' && char|0x20 <= 'z'
}

// IsDrivePath reports whether path starts with a drive letter, a colon and
// a separator of either kind (C:\ or c:/).
func IsDrivePath(path string) bool {
	return len(path) >= 3 && IsASCIILetter(path[0]) && path[1] == ':' &&
		(path[2] == '\\' || path[2] == '/')
}

// Elements returns the non-empty elements of path: split at backslashes and
// slashes for a Windows path, at slashes only for a POSIX one, where a
// backslash is a legal character of a file name.
func Elements(path string, windows bool) []string {
	return strings.FieldsFunc(path, func(char rune) bool {
		return char == '/' || windows && char == '\\'
	})
}
