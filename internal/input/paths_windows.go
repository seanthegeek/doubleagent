package input

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// parsePath splits path with this platform's rules.
func parsePath(path string) pathForm { return parseWindowsPath(path) }

// samePath reports whether two cleaned paths name one place: on Windows
// without regard to ASCII case.
func samePath(first, second string) bool { return syspath.Fold(first) == syspath.Fold(second) }

// JoinPath returns a path as given with name joined to it: name follows
// directory after a backslash unless directory is empty or already ends
// with a separator or a drive's colon; nothing else of directory changes.
func JoinPath(directory, name string) string {
	if directory == "" || strings.ContainsRune(`\/:`, rune(directory[len(directory)-1])) {
		return directory + name
	}
	return directory + `\` + name
}

// trimRightSlash removes trailing slashes.
func trimRightSlash(text string) string { return strings.TrimRight(text, "/") }

// memberSeparators returns an archive member name with this platform's
// path separators as slashes: on Windows a backslash is one too.
func memberSeparators(name string) string { return strings.ReplaceAll(name, `\`, "/") }

// memberNameAllowed reports whether this platform can hold a member name
// below a directory without it naming another place. On Windows a colon
// names a drive or an alternate data stream of a file, so a name holding
// one is refused.
func memberNameAllowed(name string) bool { return !strings.Contains(name, ":") }
