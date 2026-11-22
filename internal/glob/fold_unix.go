//go:build unix

package glob

// fold returns text unchanged: on Unix globs match in the C locale, byte for
// byte, so case counts.
func fold(text string) string { return text }
