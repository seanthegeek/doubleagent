package syspath

import "os"

// FromNative returns a path the OS or the command line handed the binary in
// the canonical form: forward slashes, no \\?\ prefix, no trailing slash.
func FromNative(path string) string { return fromWindows(path) }

// ToNative returns a canonical path in the form written to output:
// backslashes, and a bare drive with its root separator.
func ToNative(path string) string { return toWindows(path) }

// Extended returns an absolute path in the \\?\ form the binary's own Win32
// calls take, with . and .. resolved; a relative path comes back unchanged.
func Extended(path string) string { return extendedWindows(path) }

// Opened returns the path Windows will open for path, in canonical form, its
// . and .. segments removed by name as Extended removes them: the path the
// root check must judge.
func Opened(path string) string { return openedWindows(path) }

// Fold returns the key two paths are compared by: text with ASCII capitals
// lowered and every other byte kept. NTFS also ignores the case of most other
// letters, through a table written to each volume when it is formatted, but
// that table keeps apart pairs Unicode folds (ı and I, ſ and S, µ and Μ, ς
// and Σ, every pair outside the Basic Multilingual Plane), so no fold of ours
// matches it. Folding ASCII only never merges two names NTFS keeps apart;
// two spellings of one existing directory are made the same bytes by
// fsmeta.OnDiskPath before they are compared. The key is never written.
func Fold(text string) string { return foldASCII(text) }

// IsAbs reports whether path, canonical or native, starts with a drive and a
// separator, is a bare drive, or is a UNC path.
func IsAbs(path string) bool { return isAbsWindows(path) }

// ArchiveRel returns the archive form of a path below fs/: C:/a becomes /C/a,
// //server/share/a becomes /server/share/a, anything else is unchanged.
func ArchiveRel(path string) string { return archiveRelWindows(path) }

// SystemBase returns the base of the system scope: root in image mode, else
// the drive the SYSTEMDRIVE environment variable names, else C:.
func SystemBase(root string) string {
	return systemBaseWindows(root, os.Getenv("SystemDrive"))
}

// NativeErrors returns a copy of err's chain with every *fs.PathError path and
// *os.LinkError name in native form, for rows and log lines.
func NativeErrors(err error) error {
	_, rewritten := nativeErrors(err, nativeWindows)
	return rewritten
}
