package timeline

import (
	"strings"
)

// CanonicalProjectPath returns one spelling for a Windows project path:
// c:/Users/a/demo, C:\Users\a\demo\ and \\?\C:\Users\a\demo all become
// C:\Users\a\demo, with an upper-case drive letter, backslashes, no
// repeated or trailing separator and no extended-length prefix (\\?\ or
// \\?\UNC\). The case after the drive is kept, since the folder's real
// casing is unknown. Any other path, //server/share included, is returned
// unchanged, because a backslash is a legal character in a POSIX file name.
func CanonicalProjectPath(path string) string {
	if canonical, ok := windowsForm(path); ok {
		return canonical
	}
	return path
}

// ProjectKey returns the key under which the inventory groups a project
// path: a Windows form canonicalised and lower-cased in ASCII, since
// Windows file names are case-insensitive, and any other path unchanged.
func ProjectKey(path string) string {
	if canonical, ok := windowsForm(path); ok {
		return lowerASCII(canonical)
	}
	return path
}

// IsWindowsPath reports whether path is in a Windows form that
// CanonicalProjectPath rewrites: a drive letter with a separator, or a UNC
// path naming at least a server, with or without an extended-length prefix.
func IsWindowsPath(path string) bool {
	_, ok := windowsForm(path)
	return ok
}

// windowsForm returns path in the canonical Windows form and true, or false
// when path is not a Windows form.
func windowsForm(path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, `\\?\`); ok {
		path = rest
		if len(rest) >= 4 && strings.EqualFold(rest[:4], `UNC\`) {
			path = `\\` + rest[4:]
		}
	}
	if isDriveStart(path) {
		parts := pathParts(path[2:])
		return string(upperASCII(path[0])) + `:\` + strings.Join(parts, `\`), true
	}
	if rest, ok := strings.CutPrefix(path, `\\`); ok {
		if parts := pathParts(rest); len(parts) > 0 {
			return `\\` + strings.Join(parts, `\`), true
		}
	}
	return "", false
}

// isDriveStart reports whether path starts with a drive letter, a colon and
// a separator (C:\ or c:/).
func isDriveStart(path string) bool {
	return len(path) >= 3 && isASCIILetter(path[0]) && path[1] == ':' &&
		(path[2] == '\\' || path[2] == '/')
}

// pathParts returns the non-empty components of path split at both kinds
// of slash.
func pathParts(path string) []string {
	return strings.FieldsFunc(path, func(char rune) bool { return char == '\\' || char == '/' })
}

// isASCIILetter reports whether char is an ASCII letter.
func isASCIILetter(char byte) bool {
	return char|0x20 >= 'a' && char|0x20 <= 'z'
}

// upperASCII returns char in upper case when it is an ASCII letter.
func upperASCII(char byte) byte {
	if char >= 'a' && char <= 'z' {
		return char - ('a' - 'A')
	}
	return char
}

// lowerASCII returns text with its ASCII letters in lower case and every
// other byte unchanged, since a path is bytes.
func lowerASCII(text string) string {
	out := []byte(text)
	for i, char := range out {
		if char >= 'A' && char <= 'Z' {
			out[i] = char + ('a' - 'A')
		}
	}
	return string(out)
}
