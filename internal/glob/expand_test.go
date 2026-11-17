package glob

import (
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
// returns its path. Beside it lies "outside", which "../out*/o" reaches.
// addPlatformEntries adds the names and links only one platform can hold.
func buildTree(t *testing.T, rootName string) string {
	t.Helper()
	parent := t.TempDir()
	base := parent + "/" + rootName
	directories := []string{
		"", ".hidden", ".h-x", "dir/sub", "dir/sub2", "sp ace", "sp", "../outside",
	}
	for _, directory := range directories {
		if err := os.MkdirAll(base+"/"+directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := []string{
		"a.txt", "b.txt", "ab", "[ab]", "sp ace/f", "sp/f", "é", "file-not-dir",
		".h-x/auth.json", "dir/sub/file", "dir/.dot", "../outside/o",
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
	{"a bracket never matches a leading dot", "[.]h*", nil},
	{"an escaped leading dot matches dot names", `\.h*`, []string{".h-x", ".hidden"}},
	{"a literal tail is kept only when it exists", ".h*/auth.json", []string{".h-x/auth.json"}},
	{"newlines split the pattern into words", "b.txt\na.txt", []string{"b.txt", "a.txt"}},
	{"a pattern without a match gives nothing", "missing*", nil},
	{"a literal pattern that exists is kept", "dir/sub/file", []string{"dir/sub/file"}},
	{"a pattern that matches nothing is kept when its literal text exists", "[ab]",
		[]string{"[ab]"}},
	{"dot-dot after the base reaches its parent", "../out*/o", []string{"../outside/o"}},
	{"an empty pattern gives nothing", "", nil},
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
