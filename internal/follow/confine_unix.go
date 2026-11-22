//go:build unix

package follow

// NewConfinement returns no confinement on Unix: the root check's walk
// there is physical, the kernel's own resolution, and a test compares the
// two on every shape, while asking an open file for its path is not exact
// (Linux's /proc/self/fd names a deleted file with a suffix and depends on
// the mount namespace; macOS's F_GETPATH gives one of a file's hard links).
func NewConfinement(string, func(string)) (*Confinement, error) {
	return nil, nil
}
