package input

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// samePath reports whether two cleaned paths name one place: on Windows
// without regard to ASCII case.
func samePath(first, second string) bool { return syspath.Fold(first) == syspath.Fold(second) }

// isSeparator reports whether char separates the elements of a host path.
func isSeparator(char rune) bool { return char == '/' || char == '\\' }

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
