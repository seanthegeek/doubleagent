//go:build !windows

package relocation

import (
	"os"
	"testing"
)

// invalidFileName reports whether err says the file name's syntax is not
// valid on this platform, which only Windows restricts.
func invalidFileName(error) bool {
	return false
}

// newlineFixture returns the search fixture's files whose names hold a
// newline, each holding text: a file, and a file under such a directory.
func newlineFixture(text string) map[string]string {
	return map[string]string{"home/alice/scripts/d\nx": text, "home/alice/scripts/nl\ndir/f.sh": text}
}

// makeLink makes path a symlink to target.
func makeLink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
