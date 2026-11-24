package walk

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
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
// names hold. Since a match is spelled as the disk lists it, a path several
// lines reach has one spelling, and the first of those lines keeps it.
// check, nil for none, is asked before the expander looks in a directory;
// each link it refuses is one OutOfRoot match, in the place of the first
// glob that reached it.
func ExpandCatalog(base string, table []catalog.Entry, check glob.Check, opener *fsmeta.Opener,
) []Match {
	var found []Match
	seen, refused := map[string]bool{}, map[string]bool{}
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
			if !seen[path] {
				seen[path] = true
				found = append(found, Match{Agent: entry.Agent, Glob: entry.Glob, Path: path})
			}
		}
	}
	return found
}
