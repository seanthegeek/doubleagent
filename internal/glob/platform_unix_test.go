//go:build unix

package glob

import (
	"os"
	"testing"
)

// fixtureRootName holds `[` and `*`, so a base read as a pattern shows.
const fixtureRootName = "ro[o]t*"

// addPlatformEntries adds the names Windows refuses (a star, a newline),
// the symlinks the Windows runner cannot make, and a directory its mode
// makes unreadable.
func addPlatformEntries(t *testing.T, base string) {
	t.Helper()
	if err := os.Mkdir(base+"/unreadable", 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"q*", "x\ny", "nl\n", "unreadable/x"} {
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
}

// platformExpandCases are the expansions over the Unix-only entries.
var platformExpandCases = []expandCase{
	{"a star skips names starting with a dot", "*", []string{
		"[ab]", "a.txt", "ab", "b.txt", "dangling", "dir", "file-not-dir", "link-dir",
		"link-out", "nl\n", "q*", "sp", "sp ace", "unreadable", "x\ny", "é"}},
	{"a symlinked directory in the middle is followed", "link-*/sub",
		[]string{"link-dir/sub"}},
	{"a dangling link is kept", "dang*", []string{"dangling"}},
	{"a trailing slash keeps directories and links to them", "*/", []string{
		"dir/", "link-dir/", "link-out/", "sp ace/", "sp/", "unreadable/"}},
	{"a star crosses into a name ending in a newline", "nl*", []string{"nl\n"}},
	{"a star matches a newline inside a name", "x*", []string{"x\ny"}},
	{"an escaped star is literal and its escape stays", `q\*`, nil},
	{"an unreadable directory gives no match", "unreadable/*", nil},
}

// rootEntry returns an entry the root directory holds on every Unix runner.
func rootEntry(_ *testing.T) string { return "/usr" }

// platformMatchPathCases are the MatchPath cases that depend on case.
var platformMatchPathCases = []matchPathCase{
	{"a class is case-sensitive", "[a]", "A", false},
	{"a POSIX class matches", "[[:upper:]]", "Q", true},
}
