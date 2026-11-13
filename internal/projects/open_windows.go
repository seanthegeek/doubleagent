package projects

import "os"

// openNoFollow opens path read-only. Windows has no O_NOFOLLOW; openRegular's
// Lstat has already refused a symlink.
func openNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
