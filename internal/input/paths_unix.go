//go:build !windows

package input

import "strings"

// parsePath splits path with this platform's pathlib rules.
func parsePath(path string) pathForm { return parsePosixPath(path) }

// samePath reports whether two printed paths are equal as pathlib paths
// compare: byte for byte.
func samePath(first, second string) bool { return first == second }

// osJoin joins a directory and a name as Python's os.path.join does.
func osJoin(directory, name string) string {
	if directory == "" || directory[len(directory)-1] == '/' {
		return directory + name
	}
	return directory + "/" + name
}

// trimRightSlash removes trailing slashes, as str.rstrip("/").
func trimRightSlash(text string) string { return strings.TrimRight(text, "/") }

// memberSeparators returns an archive member name with this platform's
// path separators as slashes: on POSIX only the slash is one.
func memberSeparators(name string) string { return name }

// memberNameAllowed reports whether this platform can hold a member name
// below a directory without it naming another place; on POSIX every name
// can.
func memberNameAllowed(string) bool { return true }
