//go:build !windows

package parserkit

// platformName reports whether this platform holds name as one component
// below a directory; on POSIX every name Component accepts does.
func platformName(string) bool { return true }
