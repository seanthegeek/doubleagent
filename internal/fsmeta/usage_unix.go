//go:build unix

package fsmeta

import (
	"os"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// openDirectoryItself opens path to list it, never following a symlink at
// its last component and refusing anything but a directory, and has
// opener's Admit judge the handle. So a directory swapped for a symlink after
// its lstat is not listed, and on an image run a parent swapped for one that
// leads out of the root is refused by Admit.
func openDirectoryItself(opener *Opener, path string) (*os.File, error) {
	handle, err := osfile.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	return opener.admitted(path, handle)
}
