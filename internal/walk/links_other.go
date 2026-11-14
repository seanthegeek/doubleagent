//go:build !windows

package walk

// recordsLinkErrors is false where the sh collector ignores a failed ln -s:
// the symlink row keeps its archive path and no line is logged.
const recordsLinkErrors = false
