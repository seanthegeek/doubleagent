//go:build windows

package volumes

// canRead is always true: Windows has no permission bits to test.
func canRead(string) bool { return true }

// canEnter is always true: Windows has no permission bits to test.
func canEnter(string) bool { return true }
