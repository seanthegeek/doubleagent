//go:build unix

package glob

import "slices"

// sortMatches sorts a word's matches bytewise as whole relative paths, as
// the shell sorts a pattern's expansion under LC_ALL=C.
func sortMatches(matches []string) { slices.Sort(matches) }
