package walk

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
)

// Match is one catalog match: the agent and glob of its table line and the
// path matched. A Match with a Note is a case variant dropped as a second
// spelling of a path an earlier line matched; it has no path to collect, and
// the Note is its log line.
type Match struct {
	Agent string
	Glob  string
	Path  string
	Note  string
}

// ExpandCatalog returns every match of table under base, in table order, as
// expand_catalog gives them: each glob expanded inside base, a matched name
// split at its newlines into one match per line (empty ones included), and
// the matches whose paths differ only by ASCII case folded by device and
// inode, so one directory is collected once.
func ExpandCatalog(base string, table []catalog.Entry) []Match {
	var found []Match
	for _, entry := range table {
		if entry.Agent == "" || strings.HasPrefix(entry.Agent, "#") {
			continue
		}
		for _, path := range glob.Expand(base, entry.Glob) {
			for _, line := range strings.Split(path, "\n") {
				found = append(found, Match{Agent: entry.Agent, Glob: entry.Glob, Path: line})
			}
		}
	}
	return foldCaseMatches(found)
}

// foldCaseMatches applies classify_case_matches and fold_case_matches to
// found: a match whose lowered path equals an earlier one's is dropped when
// it is the same file, and a match lying inside another match's directory
// spelled differently takes that match's spelling when the two directories
// are one, so the nested claim prunes it. Only colliding paths are stat'ed.
func foldCaseMatches(found []Match) []Match {
	lowered := make([]string, len(found))
	for i, match := range found {
		lowered[i] = asciiLower(match.Path)
	}
	folded := make([]Match, 0, len(found))
	for i, match := range found {
		if same := firstEqual(lowered, i); same >= 0 {
			folded = append(folded, dropSameFile(match, found[same].Path))
		} else if parent := shortestParent(found, lowered, i); parent >= 0 {
			folded = append(folded, respell(match, found[parent].Path))
		} else {
			folded = append(folded, match)
		}
	}
	return folded
}

// firstEqual returns the first index before i whose lowered path equals
// lowered[i], or -1.
func firstEqual(lowered []string, i int) int {
	for j := 0; j < i; j++ {
		if lowered[j] == lowered[i] {
			return j
		}
	}
	return -1
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
		match.Note = fmt.Sprintf(
			"  [%s] %s: same file as %s (case-insensitive filesystem), collected there",
			match.Agent, match.Path, kept)
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

// fileID returns the device and inode of path itself, as file_id does, and
// false when path cannot be stat'ed.
func fileID(path string) ([2]uint64, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return [2]uint64{}, false
	}
	return [2]uint64{meta.Dev, meta.Ino}, true
}

// asciiLower lowers the ASCII letters of text and leaves every other byte,
// as awk's tolower does under LC_ALL=C.
func asciiLower(text string) string {
	lowered := []byte(text)
	for i, char := range lowered {
		if 'A' <= char && char <= 'Z' {
			lowered[i] = char + 'a' - 'A'
		}
	}
	return string(lowered)
}
