//go:build !windows

package analyze

// pathText returns path as Python's str(Path(path)) prints it on this
// platform.
func pathText(path string) string { return posixPathText(path) }
