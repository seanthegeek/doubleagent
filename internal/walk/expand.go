package walk

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// Match is one catalog match: the agent and glob of its table line and the
// path matched. A Match with a Note is a case variant dropped as a second
// spelling of a path an earlier line matched; it has no path to collect, and
// the Note is its log line. A Match with OutOfRoot set is no match: it names
// a link out of the image root that the expansion of its glob stopped at,
// the first line to reach that link, with no Path.
type Match struct {
	Agent     string
	Glob      string
	Path      string
	Note      string
	OutOfRoot follow.Refusal
}

// ExpandCatalog returns every match of table under base, in table order:
// each glob expanded inside base, each matched path kept whole as one value
// whatever bytes its names hold, a newline included, and the matches whose
// paths differ only by ASCII case folded by device and inode, so one
// directory is collected once. check, nil for none, is asked
// before the expander looks in a directory; each link it refuses is one
// OutOfRoot match, in the place of the first glob that reached it.
func ExpandCatalog(base string, table []catalog.Entry, check glob.Check, opener *fsmeta.Opener,
) []Match {
	var found []Match
	refused := map[string]bool{}
	for _, entry := range table {
		if entry.Agent == "" || strings.HasPrefix(entry.Agent, "#") {
			continue
		}
		paths, refusals := glob.Expand(base, entry.Glob, check, opener)
		for _, refusal := range refusals {
			if !refused[refusal.Link] {
				refused[refusal.Link] = true
				found = append(found, Match{Agent: entry.Agent, Glob: entry.Glob,
					OutOfRoot: refusal.Refusal})
			}
		}
		for _, path := range paths {
			found = append(found, Match{Agent: entry.Agent, Glob: entry.Glob, Path: path})
		}
	}
	return foldCaseMatches(found)
}

// foldCaseMatches folds the case variants of found: a match whose lowered path equals an earlier one's is dropped when
// it is the same file, and a match lying inside another match's directory
// spelled differently takes that match's spelling when the two directories
// are one, so the nested claim prunes it. Only colliding paths are stat'ed.
// An OutOfRoot match is kept as it is.
func foldCaseMatches(found []Match) []Match {
	lowered := make([]string, len(found))
	for i, match := range found {
		lowered[i] = asciiLower(match.Path)
		if match.OutOfRoot.Link != "" {
			// No path holds a NUL, so a link out of the root folds with none.
			lowered[i] = "\x00" + match.OutOfRoot.Link
		}
	}
	folded := make([]Match, 0, len(found))
	for i, match := range found {
		if match.OutOfRoot.Link != "" {
			folded = append(folded, match)
		} else if same := slices.Index(lowered[:i], lowered[i]); same >= 0 {
			folded = append(folded, dropSameFile(match, found[same].Path))
		} else if parent := shortestParent(found, lowered, i); parent >= 0 {
			folded = append(folded, respell(match, found[parent].Path))
		} else {
			folded = append(folded, match)
		}
	}
	return folded
}

// shortestParent returns the index of the shortest other match that lowered
// path i lies inside while its own spelling of that prefix differs, the
// first such match when several are as short, or -1.
func shortestParent(found []Match, lowered []string, i int) int {
	parent := -1
	for j := range found {
		size := len(lowered[j])
		if j == i || len(lowered[i]) <= size+1 || lowered[i][:size+1] != lowered[j]+"/" ||
			found[i].Path[:size] == found[j].Path {
			continue
		}
		if parent < 0 || size < len(lowered[parent]) {
			parent = j
		}
	}
	return parent
}

// dropSameFile returns match with its log note when it is the path kept or
// the same file, else match unchanged.
func dropSameFile(match Match, kept string) Match {
	identity, ok := fileID(match.Path)
	keptID, keptOK := fileID(kept)
	if match.Path == kept || (ok && keptOK && identity == keptID) {
		match.Note = variantNote(match, kept)
	}
	return match
}

// respell returns match with its first len(parent) bytes replaced by parent
// when those bytes name the same directory as parent.
func respell(match Match, parent string) Match {
	prefix := match.Path[:len(parent)]
	identity, ok := fileID(prefix)
	parentID, parentOK := fileID(parent)
	if ok && parentOK && identity == parentID {
		match.Path = parent + match.Path[len(parent):]
	}
	return match
}

// fileID returns the device and inode of path itself, and false when path cannot be stat'ed or has no identity (fsmeta.Lstat leaves
// it 0 for a Windows file it cannot open), which proves nothing: two such
// case variants stay apart.
func fileID(path string) ([2]uint64, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil || meta.Ino == 0 {
		return [2]uint64{}, false
	}
	return [2]uint64{meta.Dev, meta.Ino}, true
}

// asciiLower lowers the ASCII letters of text and leaves every other byte,
// as tolower does in the C locale.
func asciiLower(text string) string {
	lowered := []byte(text)
	for i, char := range lowered {
		if 'A' <= char && char <= 'Z' {
			lowered[i] = char + 'a' - 'A'
		}
	}
	return string(lowered)
}
