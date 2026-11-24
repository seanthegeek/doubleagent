package walk

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Match is one catalog match: the agent and glob of its table line and the
// path matched. A Match with OutOfRoot set is no match: it names a link out
// of the image root that the expansion of its glob stopped at, the first
// line to reach that link, with no Path.
type Match struct {
	Agent     string
	Glob      string
	Path      string
	OutOfRoot follow.Refusal
}

// ExpandCatalog returns every match of table under base, in table order (by
// agent, then glob, for a table catalog.Parse read): each glob expanded
// inside base, each matched path kept whole as one value whatever bytes its
// names hold. A path several lines reach is kept once, for the first of
// them (keptEntry). check, nil for none, is asked before the expander looks
// in a directory; each link it refuses is one OutOfRoot match, in the place
// of the first glob that reached it.
func ExpandCatalog(base string, table []catalog.Entry, check glob.Check, opener *fsmeta.Opener,
) []Match {
	var found []Match
	seen, refused := map[string][]string{}, map[string]bool{}
	same := follow.SameEntry(check)
	expander := glob.NewExpander(check, opener)
	for _, entry := range table {
		if entry.Agent == "" || strings.HasPrefix(entry.Agent, "#") {
			continue
		}
		paths, refusals := expander.Expand(base, entry.Glob)
		for _, refusal := range refusals {
			if !refused[refusal.Link] {
				refused[refusal.Link] = true
				found = append(found, Match{Agent: entry.Agent, Glob: entry.Glob,
					OutOfRoot: refusal.Refusal})
			}
		}
		for _, path := range paths {
			if !keptEntry(seen, path, same) {
				found = append(found, Match{Agent: entry.Agent, Glob: entry.Glob, Path: path})
			}
		}
	}
	return found
}

// keptEntry reports whether seen, the paths kept so far by their ASCII
// lowered form, holds path or a spelling of it the file system takes for the
// same entry, and adds path when not. A listed match is spelled as the disk
// lists it, but below a directory that can be entered and not listed a
// match is spelled as its line spells it, so on a disk that ignores case two
// lines reach one entry by two spellings; a directory that keeps case holds
// two entries there, which stay apart.
func keptEntry(seen map[string][]string, path string, same syspath.SameEntry) bool {
	key := asciiLower(path)
	for _, earlier := range seen[key] {
		if earlier == path || same(earlier, path) {
			return true
		}
	}
	seen[key] = append(seen[key], path)
	return false
}

// asciiLower returns text with its ASCII capitals lowered.
func asciiLower(text string) string {
	lowered := []byte(text)
	for i, char := range lowered {
		if 'A' <= char && char <= 'Z' {
			lowered[i] = char + 'a' - 'A'
		}
	}
	return string(lowered)
}
