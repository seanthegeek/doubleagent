//go:build unix

package glob

import "slices"

// sortMatches sorts a word's matches bytewise as whole relative paths, as
// the shell sorts a pattern's expansion under LC_ALL=C.
func sortMatches(matches []string) { slices.Sort(matches) }

// listsDotEntries is true: the shell's directory read returns . and .., which
// a segment starting with a dot matches.
const listsDotEntries = true
