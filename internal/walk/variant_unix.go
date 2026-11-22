//go:build unix

package walk

// variantNote is the log line of match, dropped as a second spelling of the
// path kept.
func variantNote(match Match, kept string) string {
	return "  [" + match.Agent + "] " + match.Path + ": same file as " + kept +
		" (case-insensitive filesystem), collected there"
}
