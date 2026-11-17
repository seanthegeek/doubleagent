//go:build unix

package walk

import "io/fs"

// descends reports whether the walk lists entry as a directory: one the
// directory listing types as a directory, as find descends it.
func descends(entry fs.DirEntry) bool {
	return entry.IsDir()
}
