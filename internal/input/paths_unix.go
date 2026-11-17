//go:build !windows

package input

import "strings"

// parsePath splits path with this platform's rules.
func parsePath(path string) pathForm { return parsePosixPath(path) }

// samePath reports whether two cleaned paths name one place: byte for
// byte on POSIX.
func samePath(first, second string) bool { return first == second }

// JoinPath returns a path as given with name joined to it: name follows
// directory after a separator unless directory is empty or already ends
// with one; nothing else of directory changes.
func JoinPath(directory, name string) string {
	if directory == "" || directory[len(directory)-1] == '/' {
		return directory + name
	}
	return directory + "/" + name
}

// trimRightSlash removes trailing slashes.
func trimRightSlash(text string) string { return strings.TrimRight(text, "/") }

// memberSeparators returns an archive member name with this platform's
// path separators as slashes: on POSIX only the slash is one.
func memberSeparators(name string) string { return name }

// memberNameAllowed reports whether this platform can hold a member name
// below a directory without it naming another place; on POSIX every name
// can.
func memberNameAllowed(string) bool { return true }
