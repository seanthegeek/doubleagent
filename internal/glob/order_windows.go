package glob

// sortMatches leaves a word's matches in the order they were found: each
// directory read in Get-ChildItem's order, as the PowerShell collector's
// Expand-Glob returns them unsorted.
func sortMatches(_ []string) {}
