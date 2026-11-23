package parserkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// writeEntryFiles writes each named file, holding its own name, in a new
// directory and returns the directory.
func writeEntryFiles(t *testing.T, names ...string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range names {
		if err := osfile.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func TestKeyOfTellsEntriesApart(t *testing.T) {
	directory := writeEntryFiles(t, "a.json", "b.json")
	tests := []struct {
		name          string
		first, second string
		same          bool
	}{
		{"one path is one entry", "a.json", "a.json", true},
		{"two files are two entries", "a.json", "b.json", false},
		{"a file and a missing name are two entries", "a.json", "missing.json", false},
		{"two missing names are two entries", "missing.json", "other.json", false},
		{"a file and its directory are two entries", "a.json", ".", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first := KeyOf(filepath.Join(directory, test.first))
			second := KeyOf(filepath.Join(directory, test.second))
			if (first == second) != test.same {
				t.Errorf("keys equal is %t, want %t", first == second, test.same)
			}
		})
	}
}

// opensOneFile reports whether the disk under directory opens the file
// named name by spelled too, found by trying it.
func opensOneFile(t *testing.T, directory, name, spelled string) bool {
	t.Helper()
	written, err := osfile.Lstat(filepath.Join(directory, name))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := osfile.Lstat(filepath.Join(directory, spelled))
	return err == nil && os.SameFile(written, opened)
}

// Every spelling the disk opens one file by, in ASCII case, in the case of
// an accented letter, or with that letter written as a letter and a
// combining accent, has the file's one key; a spelling the disk tells
// apart names nothing here, and the case says so.
func TestKeyOfGivesEverySpellingOfOneFileOneKey(t *testing.T) {
	const name = "\u00e9t\u00e9.json"
	tests := []struct{ name, spelled string }{
		{"ASCII capitals", "\u00e9T\u00e9.JSON"},
		{"accented capitals", "\u00c9T\u00c9.json"},
		{"one accented capital", "\u00c9t\u00e9.json"},
		{"accents written as combining marks", "e\u0301te\u0301.json"},
	}
	directory := writeEntryFiles(t, name)
	want := KeyOf(filepath.Join(directory, name))
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !opensOneFile(t, directory, name, test.spelled) {
				t.Skip("this disk tells the two spellings apart, so they open two names")
			}
			if KeyOf(filepath.Join(directory, test.spelled)) != want {
				t.Errorf("%q has a key of its own", test.spelled)
			}
		})
	}
}

// A missing name has nothing to read, so it is keyed by its own bytes: two
// spellings of it are two keys.
func TestKeyOfKeepsTwoSpellingsOfAMissingNameApart(t *testing.T) {
	directory := writeEntryFiles(t, "a.json")
	if KeyOf(filepath.Join(directory, "MISSING.JSON")) == KeyOf(filepath.Join(directory, "missing.json")) {
		t.Error("two spellings of a missing name share a key")
	}
}

func TestPerHomeKeepsOneValuePerHome(t *testing.T) {
	sets := NewPerHome(func() map[string]bool { return map[string]bool{} })
	first, second := &input.Home{DiskPath: "/h"}, &input.Home{DiskPath: "/h"}
	sets.For(first)["entry"] = true
	tests := []struct {
		name string
		home *input.Home
		want bool
	}{
		{"the same home gives the value it was given", first, true},
		{"another home of the same path starts empty", second, false},
		{"no home starts empty", nil, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sets.For(test.home)["entry"]; got != test.want {
				t.Errorf("holds the entry is %t, want %t", got, test.want)
			}
		})
	}
	sets.For(nil)["entry"] = true
	if sets.For(nil)["entry"] {
		t.Error("an artifact with no home kept a value for the next")
	}
}
