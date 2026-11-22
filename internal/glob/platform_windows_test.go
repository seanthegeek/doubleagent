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

// addPlatformEntries adds a capitalised directory tree, whose literal
// segments a lower-case pattern must find and respell.
func addPlatformEntries(t *testing.T, base string) {
	t.Helper()
	if err := os.MkdirAll(base+"/Users/Alice", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+"/Users/Alice/Notes.TXT", nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// platformExpandCases are the expansions that ignore case and respell, and
// that list matches in fsmeta.ListingOrder: directories first, then files,
// each group in the order NTFS lists names, by their upper-case form.
var platformExpandCases = []expandCase{
	{"a leading dot matches dot names but never . or .., which a listing never gives", ".*",
		[]string{".h-x", ".hidden"}},
	{"a dot-led wildcard cannot climb to the parent", ".?/out*/o", nil},
	{"a star lists directories, then files, and skips names starting with a dot", "*",
		[]string{"dir", "sp", "sp ace", "Users", "a.txt", "ab", "b.txt", "file-not-dir", "[ab]", "é"}},
	{"a trailing slash keeps directories", "*/", []string{"dir/", "sp/", "sp ace/", "Users/"}},
	{"matches keep the listing order across segments, unsorted", "sp*/f",
		[]string{"sp/f", "sp ace/f"}},
	{"a literal word is respelled as on disk", "users", []string{"Users"}},
	{"every literal segment is respelled", "users/alice/notes.txt",
		[]string{"Users/Alice/Notes.TXT"}},
	{"literal segments around a wildcard are respelled", "users/*/notes.txt",
		[]string{"Users/Alice/Notes.TXT"}},
	{"a wildcard segment ignores case and gives the listed name", "USERS/ALICE/*.txt",
		[]string{"Users/Alice/Notes.TXT"}},
	{"a star ignores the case of the rest", "*.TXT", []string{"a.txt", "b.txt"}},
	{"a class ignores case", "[A]B", []string{"ab"}},
	{"a missing literal tail gives nothing", "users/*/missing", nil},
	{"a missing literal word gives nothing", "users/missing", nil},
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

// platformMatchPathCases are the MatchPath cases that depend on case. A
// class name is lowered with the rest of the pattern, so an upper-case
// class matches nothing; no table holds a class name.
var platformMatchPathCases = []matchPathCase{
	{"a class ignores case", "[a]", "A", true},
	{"a POSIX upper-case class sees only lowered bytes", "[[:upper:]]", "Q", false},
}

func TestRespellGivesTheOnDiskCase(t *testing.T) {
	base := buildTree(t, fixtureRootName)
	cases := []struct {
		name    string
		literal string
		want    string
	}{
		{"a directory Users is matched by users", "users", "Users"},
		{"each existing segment is respelled", "users/alice/notes.txt", "Users/Alice/Notes.TXT"},
		{"a missing segment stays as written", "users/alice/missing", "Users/Alice/missing"},
		{"a trailing slash is kept", "users/alice/", "Users/Alice/"},
		{"a dot-dot segment stays", "users/../users", "Users/../Users"},
		{"a segment holding a search wildcard stays", "use?s", "use?s"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, directory := range []string{base, base + "/"} {
				if got := respell(directory, testCase.literal); got != testCase.want {
					t.Errorf("respell(%q, %q) = %q, want %q",
						directory, testCase.literal, got, testCase.want)
				}
			}
		})
	}
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
