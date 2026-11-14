//go:build unix

package glob

// fold returns text unchanged: the sh collector matches under LC_ALL=C,
// byte for byte, so case counts.
func fold(text string) string { return text }
