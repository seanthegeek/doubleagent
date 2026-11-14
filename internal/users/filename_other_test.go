//go:build !windows

package users

// invalidFileName reports whether err says the file name's syntax is not
// valid on this platform, which only Windows restricts.
func invalidFileName(error) bool {
	return false
}
