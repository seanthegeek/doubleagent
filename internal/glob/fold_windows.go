package glob

// fold returns text with ASCII capitals lowered: the PowerShell collector
// matches catalog, exclude and secret globs ignoring case. Only ASCII is
// folded, so a glob never matches as one two names NTFS keeps apart
// (syspath.Fold says why).
func fold(text string) string { return foldASCII(text) }
