//go:build unix

package glob

// respell returns literal unchanged: on Unix a name has one spelling.
func respell(_, literal string) string { return literal }
