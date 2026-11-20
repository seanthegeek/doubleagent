package parser

import (
	"path/filepath"
	"testing"

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

// matchesAnyCase reports whether the disk under directory opens one file
// for two case spellings of its name, found by trying it.
func matchesAnyCase(t *testing.T, directory string) bool {
	t.Helper()
	probe := filepath.Join(directory, "Probe")
	if err := osfile.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = osfile.Remove(probe) }()
	_, err := osfile.Lstat(filepath.Join(directory, "probe"))
	return err == nil
}

func TestFoldASCIILowersOnlyASCIICapitals(t *testing.T) {
	tests := []struct{ text, want string }{
		{"Message.JSON", "message.json"},
		{"\xc3\x89t\xc3\xa9", "\xc3\x89t\xc3\xa9"},
		{"\xff\xfeAZ@[`{", "\xff\xfeaz@[`{"},
	}
	for _, test := range tests {
		if got := foldASCII(test.text); got != test.want {
			t.Errorf("foldASCII(%q) = %q, want %q", test.text, got, test.want)
		}
	}
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

// On a disk that matches names in any case, every case spelling of a name
// opens one file and has one key; where the disk tells them apart there is
// nothing to merge, and the test says so.
func TestKeyOfGivesEveryCaseSpellingOfOneFileOneKey(t *testing.T) {
	directory := writeEntryFiles(t, "Message.json")
	if !matchesAnyCase(t, directory) {
		t.Skip("this disk tells names apart by case, so no two spellings open one file")
	}
	want := KeyOf(filepath.Join(directory, "Message.json"))
	for _, spelling := range []string{"message.json", "MESSAGE.JSON", "mEsSaGe.JsOn"} {
		if got := KeyOf(filepath.Join(directory, spelling)); got != want {
			t.Errorf("%s has its own key", spelling)
		}
	}
	if KeyOf(filepath.Join(directory, "MISSING.JSON")) == KeyOf(filepath.Join(directory, "missing.json")) {
		t.Error("two spellings of a missing name share a key")
	}
}
