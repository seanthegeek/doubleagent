package glob

import "github.com/seanthegeek/doubleagent/internal/syspath"

// fold returns the key syspath.Fold gives, each character upper-cased: the
// PowerShell collector matches catalog, exclude and secret globs ignoring
// case, and NTFS, which the globs are matched against, ignores the case of
// every letter, not only the ASCII ones.
func fold(text string) string { return syspath.Fold(text) }
