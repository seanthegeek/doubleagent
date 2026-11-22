//go:build unix

package fsmeta

// OnDiskName returns name and false: on Unix names are compared byte for
// byte, so a name has one spelling.
func OnDiskName(_, name string) (string, bool) {
	return name, false
}

// OnDiskPath returns path unchanged: on Unix a path has one spelling.
func OnDiskPath(_, path string) string { return path }
