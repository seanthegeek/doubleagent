//go:build unix

package walk

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// variantNote is the log line of match, dropped as a second spelling of the
// path kept, naming the line's glob, since its own spelling of the path can
// be the kept path's bytes.
func variantNote(match Match, kept string) string {
	return "  [" + match.Agent + "] " + match.Path + ": catalog line " + match.Glob +
		" names the same file as " + kept + " (case-insensitive filesystem), collected there"
}

// diskSpelling returns path, one file with other and differing from it only
// in ASCII case, with each component the two spell differently taken from
// its directory's listing (listedName): a literal glob segment is looked up,
// and a folding volume answers a lookup for any spelling. Components spelled
// alike are kept, so only a directory holding a differing component is read.
func diskSpelling(path, other string, check glob.Check, opener *fsmeta.Opener) string {
	components, others := strings.Split(path, "/"), strings.Split(other, "/")
	for k := range components {
		if components[k] == others[k] {
			continue
		}
		directory := strings.Join(components[:k], "/")
		if directory == "" {
			directory = "/"
		}
		components[k] = listedName(directory, components[k], check, opener)
	}
	return strings.Join(components, "/")
}

// listedName returns the name directory lists for name: the one entry equal
// to it once ASCII letters are lowered, read through glob.Names, so check
// and opener rule the read as they rule expansion. When the listing holds
// no such entry or several, or cannot be read, name comes back unchanged.
func listedName(directory, name string, check glob.Check, opener *fsmeta.Opener) string {
	found, count := name, 0
	for _, entry := range glob.Names(directory, check, opener) {
		if asciiLower(entry) == asciiLower(name) {
			found, count = entry, count+1
		}
	}
	if count != 1 {
		return name
	}
	return found
}
