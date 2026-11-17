//go:build unix

package walk

import "fmt"

// variantNote is the log line of match, dropped as a second spelling of the
// path kept, in the sh collector's words.
func variantNote(match Match, kept string) string {
	return fmt.Sprintf("  [%s] %s: same file as %s (case-insensitive filesystem), collected there",
		match.Agent, match.Path, kept)
}
