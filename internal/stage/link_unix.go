//go:build unix

package stage

import "os"

// createLink makes dest a symlink to target. A Unix link has no file or
// directory kind, so the directory flag is not needed.
func createLink(target, dest string, _ bool) error {
	return os.Symlink(target, dest)
}
