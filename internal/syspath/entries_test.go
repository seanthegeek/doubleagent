package syspath

import (
	"slices"
	"testing"
)

// fakeSame returns a SameEntry that calls two paths one entry when ids gives
// both the same identity, and that counts its calls in calls.
func fakeSame(ids map[string]int, calls *int) SameEntry {
	return func(first, second string) bool {
		*calls++
		return ids[first] != 0 && ids[first] == ids[second]
	}
}

func TestEntriesKeepApartWhatTheFileSystemDoesNotJoin(t *testing.T) {
	ids := map[string]int{"C:/Users/Alice": 1, "C:/Users/ALICE": 1, "C:/Users/alice": 2}
	cases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{"equal bytes are one entry", []string{"C:/a", "C:/a"}, []string{"C:/a"}},
		{"two spellings of one entry are one, the first spelling kept",
			[]string{"C:/Users/Alice", "C:/Users/ALICE"}, []string{"C:/Users/Alice"}},
		{"two entries differing only in case are two",
			[]string{"C:/Users/Alice", "C:/Users/alice"}, []string{"C:/Users/Alice", "C:/Users/alice"}},
		{"a spelling the file system cannot answer for is kept apart",
			[]string{"C:/Users/Alice", "C:/Users/aLICE"}, []string{"C:/Users/Alice", "C:/Users/aLICE"}},
		{"two unanswerable spellings are kept apart",
			[]string{"C:/gone/X", "C:/gone/x"}, []string{"C:/gone/X", "C:/gone/x"}},
		{"a third spelling joins the entry the file system names",
			[]string{"C:/Users/Alice", "C:/Users/alice", "C:/Users/ALICE", "C:/Users/alice"},
			[]string{"C:/Users/Alice", "C:/Users/alice"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			calls := 0
			entries := newEntries[int](foldASCII, fakeSame(ids, &calls))
			var kept []string
			for i, path := range testCase.paths {
				if entries.Add(path, i) {
					kept = append(kept, path)
				}
			}
			if !slices.Equal(kept, testCase.want) {
				t.Errorf("kept %q, want %q", kept, testCase.want)
			}
		})
	}
}

func TestEntriesIgnoringCaseAskOnEveryPlatform(t *testing.T) {
	ids := map[string]int{"/home/ALICE": 1, "/home/alice": 1, "/srv/Bob": 2, "/srv/bob": 3}
	calls := 0
	entries := NewEntriesIgnoringCase[int](fakeSame(ids, &calls))
	var kept []string
	for i, path := range []string{"/home/ALICE", "/home/alice", "/srv/Bob", "/srv/bob", "/gone/X",
		"/gone/x", "/home/ALICE"} {
		if entries.Add(path, i) {
			kept = append(kept, path)
		}
	}
	if want := []string{"/home/ALICE", "/srv/Bob", "/srv/bob", "/gone/X", "/gone/x"}; !slices.Equal(kept,
		want) || calls == 0 {
		t.Errorf("kept %q after %d questions, want %q: one entry for one identity, two for two "+
			"or for none", kept, calls, want)
	}
}

func TestEntriesFindPrefersTheExactSpelling(t *testing.T) {
	calls := 0
	entries := newEntries[string](foldASCII,
		fakeSame(map[string]int{"C:/x/Alice": 1, "C:/x/alice": 2}, &calls))
	entries.Put("C:/x/Alice", "first")
	entries.Put("C:/x/alice", "second")
	calls = 0
	if value, stored, found := entries.Find("C:/x/alice"); !found || value != "second" ||
		stored != "C:/x/alice" || calls != 0 {
		t.Errorf("Find = %q, %q, %v after %d questions; want the exact spelling's value "+
			"without asking the file system", value, stored, found, calls)
	}
}

func TestEntriesPutReplacesTheValueOfTheSameEntry(t *testing.T) {
	calls := 0
	ids := map[string]int{"C:/x/A": 1, "C:/x/a": 1}
	entries := newEntries[string](foldASCII, fakeSame(ids, &calls))
	entries.Put("C:/x/A", "first")
	entries.Put("C:/x/a", "second")
	if value, stored, _ := entries.Find("C:/x/A"); value != "second" || stored != "C:/x/A" {
		t.Errorf("Find = %q under %q; want the second value under the first spelling", value, stored)
	}
}

func TestEntriesWithoutFoldNeverAsk(t *testing.T) {
	calls := 0
	entries := newEntries[struct{}](func(text string) string { return text },
		fakeSame(map[string]int{"/a/X": 1, "/a/x": 1}, &calls))
	for _, path := range []string{"/a/X", "/a/x", "/a/X"} {
		entries.Add(path, struct{}{})
		entries.Has(path)
	}
	if calls != 0 || !entries.Has("/a/x") || entries.Has("/a/Y") {
		t.Errorf("same asked %d times; want none where the key is the bytes", calls)
	}
}

func TestCutUnderTrustsOnlyBytesOrTheFileSystem(t *testing.T) {
	calls := 0
	same := fakeSame(map[string]int{"C:/Users/Alice": 1, "C:/Users/ALICE": 1, "C:/Users/alice": 2},
		&calls)
	cases := []struct {
		name, path, dir, rest string
		found                 bool
	}{
		{"an exact prefix", "C:/Users/Alice/x", "C:/Users/Alice", "x", true},
		{"another spelling of the same directory", "C:/Users/ALICE/x", "C:/Users/Alice", "x", true},
		{"a sibling differing only in case", "C:/Users/alice/x", "C:/Users/Alice", "", false},
		{"the directory itself is not under it", "C:/Users/Alice", "C:/Users/Alice", "", false},
		{"a longer name is not under it", "C:/Users/Alicex/y", "C:/Users/Alice", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rest, found := cutUnder(foldASCII, testCase.path, testCase.dir, same)
			if found != testCase.found || (found && rest != testCase.rest) {
				t.Errorf("cutUnder = %q, %v; want %q, %v", rest, found, testCase.rest, testCase.found)
			}
		})
	}
}
