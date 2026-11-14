//go:build unix

package fsmeta

// OnDiskName returns name and false: the Unix file systems the sh collector
// walks are compared byte for byte, so a name has one spelling.
func OnDiskName(_, name string) (string, bool) {
	return name, false
}
