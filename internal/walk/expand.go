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
// path matched. A Match with OutOfRoot set is no match: it names a link out
// of the image root that the expansion of its glob stopped at, the first
// line to reach that link, with no Path.
type Match struct {
	Agent     string
	Glob      string
	Path      string
	OutOfRoot follow.Refusal
}

// ExpandCatalog returns every match of table under base, in table order:
// each glob expanded inside base, each matched path kept whole as one value
// whatever bytes its names hold, a newline included, and each path that
// several lines reach given once, spelled as the disk holds it
// (foldCaseMatches). check, nil for none, is asked before the expander, or
// the fold, looks in a directory; each link it refuses is one OutOfRoot
// match, in the place of the first glob that reached it.
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
	return foldCaseMatches(base, found, check, opener)
}

// foldCaseMatches merges the matches of found under base that name one
// path, so each path appears once, spelled as the disk holds it
// (diskSpelling, for two spellings that are one file by fileID). The match
// that represents a path is the earliest whose line's glob, under base,
// matches the disk's spelling byte for byte (globSpells), or the earliest
// when none does. A match inside another's directory is respelled, not
// merged. Only paths spelled differently are stat'ed or have a directory
// listed; an OutOfRoot match is always returned.
func foldCaseMatches(base string, found []Match, check glob.Check, opener *fsmeta.Opener,
) []Match {
	lowered := make([]string, len(found))
	for i, match := range found {
		lowered[i] = asciiLower(match.Path)
		if match.OutOfRoot.Link != "" {
			// No path holds a NUL, so a link out of the root folds with none.
			lowered[i] = "\x00" + match.OutOfRoot.Link
		}
	}
	spelled := slices.Clone(found)
	spellShared(spelled, lowered, check, opener)
	chosen := map[string]int{}
	for i, match := range spelled {
		if match.OutOfRoot.Link != "" {
			continue
		}
		first, seen := chosen[match.Path]
		if !seen || !globSpells(base, spelled[first].Glob, match.Path) &&
			globSpells(base, match.Glob, match.Path) {
			chosen[match.Path] = i
		}
	}
	merged := make([]Match, 0, len(found))
	for i, match := range spelled {
		if match.OutOfRoot.Link == "" && chosen[match.Path] != i {
			continue
		}
		merged = append(merged, match)
	}
	return merged
}

// globSpells reports whether one word of pattern, a catalog glob, names path
// below base with every byte as path spells it, ASCII case included, on
// every platform: a literal word equal to the path, or a word that matches it
// with no letter folded (glob.FoldingPattern.MatchBytes).
func globSpells(base, pattern, path string) bool {
	relative, under := strings.CutPrefix(path, base+"/")
	if !under {
		return false
	}
	for _, word := range strings.Split(pattern, "\n") {
		if word != "" && (word == relative || glob.NewFoldingPattern(word).MatchBytes(relative)) {
			return true
		}
	}
	return false
}

// spellShared respells, in place, the shared part of each pair of colliding
// matches of found that names one file, so the pair spells it alike.
func spellShared(found []Match, lowered []string, check glob.Check, opener *fsmeta.Opener) {
	for i := range found {
		for j := range i {
			size, collides := sharedPart(found, lowered, j, i)
			if !collides || !sameFile(found[j].Path[:size], found[i].Path[:size]) {
				continue
			}
			// Where the disk gives no answer the enclosing match's spelling
			// stays, or with equal paths the earlier line's.
			fallback, other := found[j].Path[:size], found[i].Path[:size]
			if len(found[i].Path) == size && len(found[j].Path) > size {
				fallback, other = other, fallback
			}
			spelling := diskSpelling(fallback, other, check, opener)
			found[j].Path = spelling + found[j].Path[size:]
			found[i].Path = spelling + found[i].Path[size:]
		}
	}
}

// sharedPart returns the length of the part matches j and i of found share
// once ASCII letters are lowered (the shorter path, whole, which the longer
// one equals or holds as a directory) and true when they spell it
// differently; false when they do not collide.
func sharedPart(found []Match, lowered []string, j, i int) (int, bool) {
	if found[j].OutOfRoot.Link != "" || found[i].OutOfRoot.Link != "" {
		return 0, false
	}
	size := min(len(lowered[j]), len(lowered[i]))
	if lowered[j][:size] != lowered[i][:size] || found[j].Path[:size] == found[i].Path[:size] {
		return 0, false
	}
	for _, path := range []string{lowered[j], lowered[i]} {
		if len(path) > size && path[size] != '/' {
			return 0, false
		}
	}
	return size, true
}

// sameFile reports whether first and second are one file by device and
// inode; false when either cannot be stat'ed, which proves nothing.
func sameFile(first, second string) bool {
	identity, ok := fileID(first)
	otherID, otherOK := fileID(second)
	return ok && otherOK && identity == otherID
}

// fileID returns the device and inode of path itself, and false when path
// cannot be stat'ed or has no identity (fsmeta.Lstat leaves it 0 for a
// Windows file it cannot open), which proves nothing: two such case variants
// stay apart.
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
