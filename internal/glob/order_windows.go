package glob

// sortMatches leaves a word's matches in the order they were found: each
// directory read in Get-ChildItem's order, as the PowerShell collector's
// Expand-Glob returns them unsorted.
func sortMatches(_ []string) {}

// listsDotEntries is false: Get-ChildItem never returns . or .., so no
// wildcard matches them, even one starting with a dot.
const listsDotEntries = false
