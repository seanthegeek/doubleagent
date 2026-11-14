//go:build !windows

package volumes

// symlinkNeedsPrivilege reports whether err says the process may not create
// symbolic links, which only Windows requires a privilege for.
func symlinkNeedsPrivilege(error) bool {
	return false
}
