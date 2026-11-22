package glob

// sortMatches leaves a word's matches in the order they were found: each
// directory read in fsmeta.ListingOrder, directories before files, unsorted.
func sortMatches(_ []string) {}

// listsDotEntries is false: a Windows directory listing never returns . or
// .., so no wildcard matches them, even one starting with a dot.
const listsDotEntries = false
