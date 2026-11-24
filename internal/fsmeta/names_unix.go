//go:build unix

package fsmeta

// OnDiskPath returns path unchanged: on Unix a path has one spelling.
func OnDiskPath(_, path string) string { return path }
