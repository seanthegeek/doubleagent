//go:build windows

package volumes

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// canRead reports whether path can be listed, as the PowerShell collector's
// Test-DirReadable decides: Windows has no permission bits to test, and a
// deny entry shows only when the listing fails.
func canRead(path string) bool {
	_, failed := fsmeta.DirReadError(path)
	return !failed
}

// canEnter is always true: Windows lets every user pass through a directory
// it cannot list.
func canEnter(string) bool { return true }
