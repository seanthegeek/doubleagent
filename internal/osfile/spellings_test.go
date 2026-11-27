package osfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// namedEntry is a listing entry of which only the name is read.
type namedEntry struct {
	fs.DirEntry
	name string
}

// Name returns the entry's name.
func (e namedEntry) Name() string { return e.name }

// listingOf returns entries holding names, in the order given.
func listingOf(names ...string) []os.DirEntry {
	entries := make([]os.DirEntry, len(names))
	for i, name := range names {
		entries[i] = namedEntry{name: name}
	}
	return entries
}

func TestASpellingIsTheListingsSingleEntryEqualUnderASCIIFold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		listing []string
		asked   string
		want    string
	}{
		{"a name the listing holds as built is taken as built", []string{"crush.db-wal", "other"}, "crush.db-wal",
			"crush.db-wal"},
		{"one entry equal under fold is taken as the listing spells it", []string{"crush.db-WAL"}, "crush.db-wal",
			"crush.db-WAL"},
		{"the exact spelling wins over a neighbour equal under fold", []string{"A.json", "a.json"}, "a.json", "a.json"},
		{"two entries equal under fold give the name as built", []string{"A.json", "a.JSON"}, "a.json", "a.json"},
		{"a letter outside ASCII is not folded", []string{"été.json"}, "ÉTÉ.json",
			"ÉTÉ.json"},
		{"an ASCII letter beside one outside ASCII is folded", []string{"éTé.json"}, "été.JSON",
			"éTé.json"},
		{"a name no entry matches is taken as built", []string{"b.json"}, "a.json", "a.json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := SpellingsOf(listingOf(test.listing...)).Spelling(test.asked); got != test.want {
				t.Errorf("Spelling(%s) = %s, want %s", test.asked, got, test.want)
			}
		})
	}
}

func TestADirectoryThatCannotBeListedGivesTheNameAsBuilt(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "missing")
	if spellings := ListSpellings(missing); spellings != nil {
		t.Errorf("ListSpellings of a missing directory = %v, want nil", spellings)
	}
	if got := SpellingIn(missing, "A.json"); got != "A.json" {
		t.Errorf("SpellingIn a missing directory = %s, want A.json", got)
	}
}

func TestSpellingInReadsTheDirectorysListing(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Sessions.json"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := SpellingIn(directory, "sessions.json"); got != "Sessions.json" {
		t.Errorf("SpellingIn = %s, want Sessions.json", got)
	}
}
