//go:build unix

package syspath

import "strings"

// FromNative returns path in the canonical form; on Unix it is unchanged.
func FromNative(path string) string { return path }

// ToNative returns path in the form written to output; on Unix it is
// unchanged.
func ToNative(path string) string { return path }

// Extended returns path in the form the binary's own system calls take; on
// Unix it is unchanged.
func Extended(path string) string { return path }

// Fold returns the key two paths are compared by; on Unix, whose filesystems
// are compared byte for byte here as the sh collector compares them, it is
// text unchanged.
func Fold(text string) string { return text }

// matchPrefix reports whether path starts with prefix byte for byte, and the
// byte index in path where the rest begins.
func matchPrefix(path, prefix string) (restIndex int, found bool) {
	if !strings.HasPrefix(path, prefix) {
		return 0, false
	}
	return len(prefix), true
}

// IsAbs reports whether path starts with /.
func IsAbs(path string) bool { return strings.HasPrefix(path, "/") }

// ArchiveRel returns the archive form of a path below fs/; on Unix it is
// unchanged.
func ArchiveRel(path string) string { return path }

// SystemBase returns the base of the system scope: root, which is empty on a
// live run since the system files sit under /.
func SystemBase(root string) string { return root }

// NativeErrors returns err with its paths in native form; on Unix it is err.
func NativeErrors(err error) error { return err }
