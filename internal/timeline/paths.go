package timeline

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
)

// CanonicalProjectPath returns one spelling for a Windows project path, a
// path as the evidence's machine wrote it (evidencepath):
// c:/Users/a/demo, C:\Users\a\demo\ and \\?\C:\Users\a\demo all become
// C:\Users\a\demo, with an upper-case drive letter, backslashes, no
// repeated or trailing separator and no extended-length prefix (\\?\ or
// \\?\UNC\). The case after the drive is kept, since the folder's real
// casing is unknown. Any other path, //server/share included, is returned
// unchanged, as is a UNC path whose server is "?" with more after it, which
// no Windows program writes and which would read as an extended-length
// prefix; CanonicalProjectPath and ProjectKey give their results back.
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
// when path is not a Windows form. A backslash is a legal character in a
// POSIX file name, so a path is read as a Windows one only in these forms.
func windowsForm(path string) (string, bool) {
	if rest, ok := strings.CutPrefix(path, `\\?\`); ok {
		path = rest
		if len(rest) >= 4 && strings.EqualFold(rest[:4], `UNC\`) {
			path = `\\` + rest[4:]
		}
	}
	if evidencepath.IsDrivePath(path) {
		parts := evidencepath.Elements(path[2:], true)
		return string(upperASCII(path[0])) + `:\` + strings.Join(parts, `\`), true
	}
	if rest, ok := strings.CutPrefix(path, `\\`); ok {
		parts := evidencepath.Elements(rest, true)
		if len(parts) == 1 || len(parts) > 1 && parts[0] != "?" {
			return `\\` + strings.Join(parts, `\`), true
		}
	}
	return "", false
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
