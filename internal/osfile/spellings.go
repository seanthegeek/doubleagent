package osfile

import (
	"os"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// Spellings is a directory's listing keyed by each name with its ASCII
// letters lowered, giving the name as the listing spells it, or "" when two
// names of the listing are one under that fold. Letters outside ASCII are
// never folded: a file system's own case table keeps apart pairs another
// fold would merge, and merging two names loses one. A nil Spellings is a
// listing that could not be read.
type Spellings map[string]string

// SpellingsOf returns the Spellings of a directory's entries.
func SpellingsOf(entries []os.DirEntry) Spellings {
	spellings := make(Spellings, len(entries))
	for _, entry := range entries {
		key := textlines.LowerASCII(entry.Name())
		if _, twice := spellings[key]; twice {
			spellings[key] = ""
		} else {
			spellings[key] = entry.Name()
		}
	}
	return spellings
}

// ListSpellings lists directory (ReadDir) and returns its Spellings, or nil
// when the listing fails, even part way, so that every name is then taken
// as it was built.
func ListSpellings(directory string) Spellings {
	entries, err := ReadDir(directory)
	if err != nil {
		return nil
	}
	return SpellingsOf(entries)
}

// Spelling returns the name to open for name in the listed directory: the
// listing's single entry equal to name under ASCII fold, spelled as the
// listing spells it (name itself when the listing holds name's bytes and no
// other entry folds to them), else name as built: when no entry, or two, are
// equal to it, or the directory could not be listed.
func (s Spellings) Spelling(name string) string {
	if spelled := s[textlines.LowerASCII(name)]; spelled != "" {
		return spelled
	}
	return name
}
