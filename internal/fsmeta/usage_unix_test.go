//go:build unix

package fsmeta

import (
	"os"
	"testing"
)

// replaceWithLink makes link, a path that no longer exists, a symlink to
// target.
func replaceWithLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
