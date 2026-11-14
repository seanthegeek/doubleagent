package glob

// fold returns text with ASCII capitals lowered: the PowerShell collector
// matches catalog, exclude and secret globs ignoring case, as the Windows
// filesystem names files.
func fold(text string) string { return foldASCII(text) }
