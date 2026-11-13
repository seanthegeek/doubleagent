//go:build unix

package projects

import (
	"os"
	"syscall"
)

// openNoFollow opens path read-only, failing when its last component is a
// symlink; O_NONBLOCK keeps a pipe swapped in after the type check from
// blocking the open.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
