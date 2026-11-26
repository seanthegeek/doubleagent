//go:build unix

package projects

import (
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/glob"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// diskSpeller spells the path of a drive value as an image on a Linux or
// macOS disk holds it. It keeps every directory's listing (glob.Expander)
// for one Discover call, so each directory is listed once however many
// values name it.
type diskSpeller struct {
	listings *glob.Expander
}

// newDiskSpeller returns a diskSpeller that asks check (the image root's)
// before it lists a directory and lists through opener.
func newDiskSpeller(check RootExit, opener *fsmeta.Opener) *diskSpeller {
	return &diskSpeller{listings: glob.NewExpander(check, opener)}
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

// diskName returns the name of the entry part names in directory and whether
// there is one. The entry with part's exact bytes wins; failing one, a
// listing read only in part lets lstat find part as written; failing that,
// the one entry equal to part ignoring the case of any letter
// (strings.EqualFold), since a part that matches too much only finds one
// more candidate and the result is the listing's own spelling
// (foldedName). Several such entries, `.`, `..` and an empty part name
// nothing.
func (speller *diskSpeller) diskName(directory, part string) (string, bool) {
	if part == "" || part == "." || part == ".." {
		return "", false
	}
	names, complete := speller.listings.Names(directory)
	for _, name := range names {
		if name == part {
			return name, true
		}
	}
	if !complete {
		if _, err := osfile.Lstat(directory + "/" + part); err == nil {
			return part, true
		}
	}
	match, matches := "", 0
	for _, name := range names {
		if foldedName(part, name) {
			match, matches = name, matches+1
		}
	}
	return match, matches == 1
}

// foldedName reports whether part and name are equal ignoring the case of
// any letter (strings.EqualFold). EqualFold reads every byte that is not
// valid UTF-8 as one replacement character, so a part or a name that is not
// valid UTF-8 is never equal by this test, only by its exact bytes.
func foldedName(part, name string) bool {
	return utf8.ValidString(part) && utf8.ValidString(name) && strings.EqualFold(part, name)
}
