//go:build unix

package relocation

import "io/fs"

// sweepOrder returns the names of a directory's entries in the order the
// sweep visits them on Unix: the listing's.
func sweepOrder(entries []fs.DirEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}
