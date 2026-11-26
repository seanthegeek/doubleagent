//go:build !windows

package input

import (
	"os"
	"path/filepath"
)

// memberSeparators returns an archive member name with this platform's
// path separators as slashes: on POSIX only the slash is one.
func memberSeparators(name string) string { return name }

// memberNameAllowed reports whether this platform can hold a name as one
// component below a directory without it naming another place; on POSIX
// every name without a slash can, a backslash included.
func memberNameAllowed(string) bool { return true }

// absolute returns path cleaned, joined to the working directory first when
// it does not start with a slash.
func absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	working, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(working, path), nil
}
