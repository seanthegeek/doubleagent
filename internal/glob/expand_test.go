package glob

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// buildTree makes the fixture tree under a fresh directory named rootName and
// returns its path. Beside it lies "outside", which the link-out link names.
func buildTree(t *testing.T, rootName string) string {
	t.Helper()
	parent := t.TempDir()
	base := parent + "/" + rootName
	directories := []string{
		"", ".hidden", ".h-x", "dir/sub", "dir/sub2", "sp ace", "sp", "unreadable", "../outside",
	}
	for _, directory := range directories {
		if err := os.MkdirAll(base+"/"+directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := []string{
		"a.txt", "b.txt", "ab", "[ab]", "q*", "x\ny", "nl\n", "sp ace/f", "sp/f", "é",
		"file-not-dir", ".h-x/auth.json", "dir/sub/file", "dir/.dot", "unreadable/x",
		"../outside/o",
	}
	for _, file := range files {
		if err := os.WriteFile(base+"/"+file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	links := [][2]string{{"dir", "link-dir"}, {"../outside", "link-out"}, {"missing", "dangling"}}
	for _, link := range links {
		if err := os.Symlink(link[0], base+"/"+link[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(base+"/unreadable", 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(base+"/unreadable", 0o755) })
	return base
}

func TestExpand(t *testing.T) {
	base := buildTree(t, "ro[o]t*")
	cases := []struct {
		name    string
		pattern string
		want    []string
	}{
		{"a star skips names starting with a dot", "*", []string{
			"[ab]", "a.txt", "ab", "b.txt", "dangling", "dir", "file-not-dir", "link-dir",
			"link-out", "nl\n", "q*", "sp", "sp ace", "unreadable", "x\ny", "é"}},
		{"a leading dot matches dot names, . and .. included", ".*",
			[]string{".", "..", ".h-x", ".hidden"}},
		{"a bracket never matches a leading dot", "[.]h*", nil},
		{"an escaped leading dot matches dot names", `\.h*`, []string{".h-x", ".hidden"}},
		{"a literal tail is kept only when it exists", ".h*/auth.json", []string{".h-x/auth.json"}},
		{"a symlinked directory in the middle is followed", "link-*/sub",
			[]string{"link-dir/sub"}},
		{"a dangling link is kept", "dang*", []string{"dangling"}},
		{"a trailing slash keeps directories and links to them", "*/", []string{
			"dir/", "link-dir/", "link-out/", "sp ace/", "sp/", "unreadable/"}},
		{"a star crosses into a name ending in a newline", "nl*", []string{"nl\n"}},
		{"a star matches a newline inside a name", "x*", []string{"x\ny"}},
		{"newlines split the pattern into words", "b.txt\na.txt", []string{"b.txt", "a.txt"}},
		{"matches sort bytewise as whole paths, so a space sorts before a slash",
			"sp*/f", []string{"sp ace/f", "sp/f"}},
		{"a pattern without a match gives nothing", "missing*", nil},
		{"a literal pattern that exists is kept", "dir/sub/file", []string{"dir/sub/file"}},
		{"a pattern that matches nothing is kept when its literal text exists", "[ab]",
			[]string{"[ab]"}},
		{"an escaped star is literal and its escape stays", `q\*`, nil},
		{"an unreadable directory gives no match", "unreadable/*", nil},
		{"dot-dot after the base reaches its parent", "../out*/o", []string{"../outside/o"}},
		{"an empty pattern gives nothing", "", nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var want []string
			for _, match := range testCase.want {
				want = append(want, base+"/"+match)
			}
			if got := Expand(base, testCase.pattern); !slices.Equal(got, want) {
				t.Errorf("Expand(%q) = %q, want %q", testCase.pattern, got, want)
			}
		})
	}
}

func TestExpandKeepsTheBaseLiteral(t *testing.T) {
	base := buildTree(t, "ro[o]t*")
	if got := Expand(base, "a.*"); !slices.Equal(got, []string{base + "/a.txt"}) {
		t.Errorf("a base holding [ and * was read as a pattern: %q", got)
	}
	if got := Expand(filepath.Dir(base)+"/missing", "*"); got != nil {
		t.Errorf("a missing base gave %q", got)
	}
}

func TestExpandOfAnEmptyBaseStartsAtTheRoot(t *testing.T) {
	got := Expand("", "*")
	if !slices.Contains(got, "/usr") {
		t.Errorf("Expand(\"\", \"*\") = %q, want the root's entries as /name", got)
	}
}
