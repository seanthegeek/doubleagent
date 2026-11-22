package relocation

import "io/fs"

// sweepOrder returns the names of a directory's entries in the order the
// sweep reaches them on Windows (stackOrder).
func sweepOrder(entries []fs.DirEntry) []string {
	return stackOrder(entries)
}
