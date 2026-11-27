//go:build unix

package glob

import (
	"os"
	"testing"
)

// fixtureRootName holds `[` and `*`, so a base read as a pattern shows.
const fixtureRootName = "ro[o]t*"

// addPlatformEntries adds the names Windows refuses (a star, a newline),
// the symlinks the Windows runner cannot make, a directory its mode makes
// unreadable and one that can be entered but not listed.
func addPlatformEntries(t *testing.T, base string) {
	t.Helper()
	for _, directory := range []string{"unreadable", "sealed"} {
		if err := os.Mkdir(base+"/"+directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"q*", "x\ny", "nl\n", "unreadable/x", "sealed/kept"} {
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
	for directory, mode := range map[string]os.FileMode{"unreadable": 0, "sealed": 0o311} {
		if err := os.Chmod(base+"/"+directory, mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(base+"/"+directory, 0o755) })
	}
}

// platformExpandCases are the expansions over the Unix-only entries.
var platformExpandCases = []expandCase{
	{"a star skips names starting with a dot and sorts bytewise", "*", []string{
		"Users", "a.txt", "ab", "b.txt", "dangling", "dir", "file-not-dir", "link-dir",
		"link-out", "nl\n", "q*", "sealed", "sp", "sp ace", "unreadable", "x\ny", "x[", "é"}},
	{"a symlinked directory in the middle is followed", "link-*/sub",
		[]string{"link-dir/sub"}},
	{"a dangling link is kept", "dang*", []string{"dangling"}},
	{"a star crosses into a name ending in a newline", "nl*", []string{"nl\n"}},
	{"a star matches a newline inside a name", "x*", []string{"x\ny", "x["}},
	{"an unreadable directory gives no match", "unreadable/*", nil},
	{"a directory that cannot be entered gives no literal", "unreadable/x", nil},
	{"a directory that cannot be listed answers a literal by lookup", "sealed/kept",
		[]string{"sealed/kept"}},
	{"a directory that cannot be listed gives no wildcard match", "sealed/*", nil},
}

// rootEntry returns an entry the root directory holds on every Unix runner.
func rootEntry(_ *testing.T) string { return "/usr" }
