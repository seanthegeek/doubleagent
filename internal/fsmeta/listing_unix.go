//go:build unix

package fsmeta

import "io/fs"

// ListingOrder returns entries unchanged: find visits a directory's entries
// in the order the directory listing returns them.
func ListingOrder(entries []fs.DirEntry) []fs.DirEntry {
	return entries
}
