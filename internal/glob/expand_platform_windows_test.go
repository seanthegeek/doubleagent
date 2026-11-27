package glob

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fixtureRootName holds `[`, so a base read as a pattern shows; Windows
// refuses a `*` in a name.
const fixtureRootName = "ro[o]t"

// addPlatformEntries adds nothing: the shared tree holds every name
// Windows accepts.
func addPlatformEntries(_ *testing.T, _ string) {}

// platformExpandCases are the expansions over the names Windows lists.
var platformExpandCases = []expandCase{
	{"a star skips names starting with a dot and sorts bytewise", "*",
		[]string{"Users", "a.txt", "ab", "b.txt", "dir", "file-not-dir", "sp", "sp ace", "x[", "é"}},
	{"a star ignores the case of the rest", "*.TXT", []string{"a.txt", "b.txt"}},
}

// rootEntry returns the first entry, by Expand's rules, of the root of the
// current directory's drive, which an empty base names on Windows.
func rootEntry(t *testing.T) string {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.VolumeName(working) + `\`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, "/"+entry.Name())
		}
	}
	if len(names) == 0 {
		t.Fatal("the drive root lists no entry")
	}
	slices.Sort(names)
	return names[0]
}

func TestNamesEndingInADotOrASpaceAreListedAsTheirOwn(t *testing.T) {
	base := strings.ReplaceAll(t.TempDir(), `\`, "/")
	// Win32 trims a trailing dot or space unless the path has the \\?\
	// prefix, so the fixture is written through that form.
	for _, file := range []string{"t/x./a", "t/x/b", "t/y ", "t/y"} {
		path := `\\?\` + strings.ReplaceAll(base+"/"+file, "/", `\`)
		if err := os.MkdirAll(path[:strings.LastIndexByte(path, '\\')], 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, pattern string
		want          []string
	}{
		{"a wildcard below x. lists x.'s own entries", "t/x./*", []string{"t/x./a"}},
		{"a wildcard below x lists x's own entries", "t/x/*", []string{"t/x/b"}},
		{"a literal x. path exists only as x.'s entry", "t/x./b", nil},
		{"a literal name ending in a space is its own file", "t/y ", []string{"t/y "}},
		{"a star lists every spelling", "t/*", []string{"t/x", "t/x.", "t/y", "t/y "}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, match := range expandAll(base, test.pattern) {
				got = append(got, strings.TrimPrefix(match, base+"/"))
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("Expand(%q) = %q, want %q", test.pattern, got, test.want)
			}
		})
	}
}
