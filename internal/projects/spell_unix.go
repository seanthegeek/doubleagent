//go:build unix

package projects

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// diskSpeller spells the path of a drive value as an image on a Linux or
// macOS disk holds it. It lists each directory once per Discover call
// (list, the glob expander's Names) and keeps an index of it, so a lookup
// costs the same however many entries the directory holds.
type diskSpeller struct {
	list    func(directory string) (names []string, complete bool)
	indexes map[string]*nameIndex
}

// nameIndex is one directory's names: the exact names, the valid UTF-8
// names by their foldKey, and whether the listing was read whole.
type nameIndex struct {
	exact    map[string]bool
	folded   map[string][]string
	complete bool
}

// newDiskSpeller returns a diskSpeller that asks check (the image root's)
// before it lists a directory and lists through opener.
func newDiskSpeller(check RootExit, opener *fsmeta.Opener) *diskSpeller {
	return &diskSpeller{list: glob.NewExpander(check, opener).Names, indexes: map[string]*nameIndex{}}
}

// foldKey returns text, valid UTF-8, with each rune replaced by the smallest
// rune of its unicode.SimpleFold orbit. strings.EqualFold takes two strings
// as equal when they have as many runes and each pair shares an orbit, so
// two keys are equal exactly when EqualFold says the strings are. EqualFold
// reads every byte that is not valid UTF-8 as one replacement character, so
// such a name is never looked up by key, only by its exact bytes.
func foldKey(text string) string {
	var key strings.Builder
	for _, char := range text {
		smallest := char
		for next := unicode.SimpleFold(char); next != char; next = unicode.SimpleFold(next) {
			smallest = min(smallest, next)
		}
		key.WriteRune(smallest)
	}
	return key.String()
}

// index returns directory's nameIndex, built from its listing the first time.
func (speller *diskSpeller) index(directory string) *nameIndex {
	if found, known := speller.indexes[directory]; known {
		return found
	}
	names, complete := speller.list(directory)
	built := &nameIndex{exact: map[string]bool{}, folded: map[string][]string{}, complete: complete}
	for _, name := range names {
		built.exact[name] = true
		if utf8.ValidString(name) {
			key := foldKey(name)
			built.folded[key] = append(built.folded[key], name)
		}
	}
	speller.indexes[directory] = built
	return built
}

// diskName returns the name of the entry part names in directory and whether
// there is one. The entry with part's exact bytes wins; failing one, a
// listing read only in part lets lstat find part as written; failing that,
// the one valid UTF-8 entry equal to part ignoring the case of any letter
// (foldKey), since a part that matches too much only finds one more
// candidate and the result is the listing's own spelling. Several such
// entries, `.`, `..` and an empty part name nothing.
func (speller *diskSpeller) diskName(directory, part string) (string, bool) {
	if part == "" || part == "." || part == ".." {
		return "", false
	}
	listed := speller.index(directory)
	if listed.exact[part] {
		return part, true
	}
	if !listed.complete {
		if _, err := osfile.Lstat(directory + "/" + part); err == nil {
			return part, true
		}
	}
	if !utf8.ValidString(part) {
		return "", false
	}
	matches := listed.folded[foldKey(part)]
	if len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}

// spell returns path with each existing part below root spelled as its
// directory lists it (diskName), stopping after the first part that is not
// a directory, a link included, and at the first missing part; the rest and
// root stay as written. A directory the root check refuses names nothing.
func (speller *diskSpeller) spell(root, path string) string {
	rest, found := strings.CutPrefix(path, root+"/")
	if !found || rest == "" {
		return path
	}
	directory := root
	parts := strings.Split(rest, "/")
	for i, part := range parts {
		name, exists := speller.diskName(directory, part)
		if !exists {
			break
		}
		parts[i] = name
		directory += "/" + name
		if info, err := osfile.Lstat(directory); err != nil || !info.IsDir() {
			break
		}
	}
	return root + "/" + strings.Join(parts, "/")
}
