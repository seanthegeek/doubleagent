package walk

import "github.com/seanthegeek/doubleagent/internal/syspath"

// variantNote is the log line of match, dropped as a second spelling of a
// path an earlier line matched, naming the line's glob rather than the path
// kept.
func variantNote(match Match, _ string) string {
	return "  [" + match.Agent + "] " + syspath.ToNative(match.Path) + ": catalog line " +
		match.Glob + " names the same path as an earlier line" +
		" (case-insensitive filesystem), collected there"
}
