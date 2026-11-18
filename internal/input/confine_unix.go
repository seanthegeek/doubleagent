//go:build !windows

package input

import "io/fs"

// isLink reports whether info describes a symbolic link.
func isLink(info fs.FileInfo) bool { return info.Mode()&fs.ModeSymlink != 0 }
