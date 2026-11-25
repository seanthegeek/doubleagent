package glob

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// expandCase is one pattern expanded inside the fixture tree and the
// matches it gives, relative to the tree's root.
type expandCase struct {
	name    string
	pattern string
	want    []string
}

// buildTree makes the fixture tree under a fresh directory named rootName and
// returns its path. Beside it lies "outside", which "../out*/o" would reach.
// addPlatformEntries adds the names and links only one platform can hold.
func buildTree(t *testing.T, rootName string) string {
	t.Helper()
	parent := t.TempDir()
	base := parent + "/" + rootName
	directories := []string{
		"", ".hidden", ".h-x", "dir/sub", "dir/sub2", "sp ace", "sp", "Users/Alice", "../outside",
	}
	for _, directory := range directories {
		if err := os.MkdirAll(base+"/"+directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := []string{
		"a.txt", "b.txt", "ab", "x[", "sp ace/f", "sp/f", "é", "file-not-dir",
		".h-x/auth.json", "dir/sub/file", "dir/.dot", "Users/Alice/Notes.TXT", "../outside/o",
	}
	for _, file := range files {
		if err := os.WriteFile(base+"/"+file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	addPlatformEntries(t, base)
	return base
}

// commonExpandCases are the expansions every platform gives alike over the
// shared part of the fixture tree.
var commonExpandCases = []expandCase{
	{"a list never matches a leading dot", "[.]h*", nil},
	{"a leading dot matches dot names but never . or ..", ".*", []string{".h-x", ".hidden"}},
	{"a literal tail is kept only when it exists", ".h*/auth.json", []string{".h-x/auth.json"}},
	{"a pattern without a match gives nothing", "missing*", nil},
	{"a literal pattern that exists is kept", "dir/sub/file", []string{"dir/sub/file"}},
	{"a literal segment is spelled as its directory lists it", "users/alice/notes.txt",
		[]string{"Users/Alice/Notes.TXT"}},
	{"a wildcard segment ignores case and gives the listed name", "USERS/*/*.txt",
		[]string{"Users/Alice/Notes.TXT"}},
	{"a missing literal gives nothing", "users/missing", nil},
	{"matches sort bytewise as whole paths, so a space sorts before a slash", "sp*/f",
		[]string{"sp ace/f", "sp/f"}},
	{"dot-dot is no listed name", "../out*/o", nil},
	{"an empty pattern gives nothing", "", nil},
	{"a trailing slash matches nothing", "dir/", nil},
	{"an unclosed bracket matches nothing", "x[", nil},
}

func TestExpand(t *testing.T) {
	base := buildTree(t, fixtureRootName)
	for _, testCase := range append(slices.Clone(commonExpandCases), platformExpandCases...) {
		t.Run(testCase.name, func(t *testing.T) {
			var want []string
			for _, match := range testCase.want {
				want = append(want, base+"/"+match)
			}
			if got := expandAll(base, testCase.pattern); !slices.Equal(got, want) {
				t.Errorf("Expand(%q) = %q, want %q", testCase.pattern, got, want)
			}
		})
	}
}

func TestOneLiteralMatchesEachSpellingACaseSensitiveDirectoryHolds(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"Goose", "goose"} {
		err := os.Mkdir(base+"/"+name, 0o755)
		if errors.Is(err, fs.ErrExist) {
			t.Skip("the test directory folds case, so it cannot hold both spellings")
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	want := []string{base + "/Goose", base + "/goose"}
	for _, pattern := range []string{"goose", "GOOSE", "g*"} {
		if got := expandAll(base, pattern); !slices.Equal(got, want) {
			t.Errorf("Expand(%q) = %q, want both spellings %q", pattern, got, want)
		}
	}
}

func TestALiteralMatchesTheNamesAPartialListingRead(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"Goose", "kept"} {
		if err := os.Mkdir(base+"/"+name, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, pattern string
		want          []string
	}{
		{"a name read is matched in another case and keeps the disk's spelling", "goose",
			[]string{"Goose"}},
		{"a name read under the line's own spelling is one match", "Goose", []string{"Goose"}},
		{"a name the listing never reached is found by lookup", "kept", []string{"kept"}},
		{"a name neither read nor on disk gives nothing", "missing", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var want []string
			for _, match := range testCase.want {
				want = append(want, base+"/"+match)
			}
			if got := expandPartly(base, []string{"Goose"}, testCase.pattern); !slices.Equal(got, want) {
				t.Errorf("Expand(%q) = %q, want %q", testCase.pattern, got, want)
			}
		})
	}
}

func TestAPartialListingStillFindsASpellingItDidNotRead(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"Goose", "goose"} {
		err := os.Mkdir(base+"/"+name, 0o755)
		if errors.Is(err, fs.ErrExist) {
			t.Skip("the test directory folds case, so it cannot hold both spellings")
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	want := []string{base + "/Goose", base + "/goose"}
	if got := expandPartly(base, []string{"Goose"}, "goose"); !slices.Equal(got, want) {
		t.Errorf("Expand(goose) = %q, want both entries %q", got, want)
	}
}

// expandPartly expands pattern in base as if base's listing had failed
// after reading names, which only a damaged file system gives for real.
func expandPartly(base string, names []string, pattern string) []string {
	expander := NewExpander(nil, nil)
	expander.listings[base] = listing{names: names}
	matches, _ := expander.Expand(base, pattern)
	return matches
}

func TestExpandKeepsTheBaseLiteral(t *testing.T) {
	base := buildTree(t, fixtureRootName)
	if got := expandAll(base, "a.*"); !slices.Equal(got, []string{base + "/a.txt"}) {
		t.Errorf("a base holding [ was read as a pattern: %q", got)
	}
	if got := expandAll(filepath.Dir(base)+"/missing", "*"); got != nil {
		t.Errorf("a missing base gave %q", got)
	}
}

func TestExpandOfAnEmptyBaseStartsAtTheRoot(t *testing.T) {
	want := rootEntry(t)
	if got := expandAll("", "*"); !slices.Contains(got, want) {
		t.Errorf("expandAll(\"\", \"*\") = %q, want the root's entries as /name, %q among them",
			got, want)
	}
}

// expandAll is Expand without a check, as a live run calls it.
func expandAll(base, pattern string) []string {
	matches, _ := Expand(base, pattern, nil, nil)
	return matches
}
