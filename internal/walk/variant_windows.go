package walk

import (
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// variantNote is the log line of match, dropped as a second spelling of a
// path an earlier line matched, naming the line's glob rather than the path
// kept.
func variantNote(match Match, _ string) string {
	return "  [" + match.Agent + "] " + syspath.ToNative(match.Path) + ": catalog line " +
		match.Glob + " names the same path as an earlier line" +
		" (case-insensitive filesystem), collected there"
}

// diskSpelling returns path unchanged: on Windows the expander already
// spells each literal segment as its directory holds it (fsmeta.OnDiskName),
// so two spellings that remain are ones the disk could not settle.
func diskSpelling(path, _ string, _ glob.Check, _ *fsmeta.Opener) string {
	return path
}
