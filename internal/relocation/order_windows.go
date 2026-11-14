package relocation

import "io/fs"

// sweepOrder returns the names of a directory's entries in the order the
// PowerShell collector's Get-SweepFiles reaches them (stackOrder).
func sweepOrder(entries []fs.DirEntry) []string {
	return stackOrder(entries)
}
