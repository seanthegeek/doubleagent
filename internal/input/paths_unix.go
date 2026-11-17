//go:build !windows

package input

import "strings"

// samePath reports whether two cleaned paths name one place: byte for
// byte on POSIX.
func samePath(first, second string) bool { return first == second }

// isSeparator reports whether char separates the elements of a host path.
func isSeparator(char rune) bool { return char == '/' }

// trimRightSlash removes trailing slashes.
func trimRightSlash(text string) string { return strings.TrimRight(text, "/") }

// memberSeparators returns an archive member name with this platform's
// path separators as slashes: on POSIX only the slash is one.
func memberSeparators(name string) string { return name }

// memberNameAllowed reports whether this platform can hold a member name
// below a directory without it naming another place; on POSIX every name
// can.
func memberNameAllowed(string) bool { return true }
