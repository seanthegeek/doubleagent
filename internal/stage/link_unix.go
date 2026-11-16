//go:build unix

package stage

import "os"

// refusesDotted is whether Dest refuses a path with a `.` or `..` component:
// not on Unix, where the sh collector stages it with them left in.
const refusesDotted = false

// createLink makes dest a symlink to target. A Unix link has no file or
// directory kind, so the directory flag is not needed.
func createLink(target, dest string, _ bool) error {
	return os.Symlink(target, dest)
}
