//go:build darwin || linux

package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// foldingStage returns a staging directory with its fs directory made, and
// skips the test where that directory tells letter case apart.
func foldingStage(t *testing.T) string {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parent+"/stage/fs", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent+"/stage/fs/probe", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(parent + "/stage/fs/PROBE"); err != nil {
		t.Skipf("%s tells letter case apart, so no two entries share a staged path", parent)
	}
	if err := os.Remove(parent + "/stage/fs/probe"); err != nil {
		t.Fatal(err)
	}
	return parent
}

func TestNoCopyOverwritesAnotherEntrysCopyInAFoldingStage(t *testing.T) {
	parent := foldingStage(t)
	// Two images, each holding one of a pair that a case-sensitive volume
	// keeps apart, staged into one directory, as two roots' paths that
	// differ only in case would be.
	first := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/one"}
	second := first
	second.Root = parent + "/two"
	sources := map[string]string{parent + "/one/Users/Alice/notes.txt": "Alice's notes",
		parent + "/two/Users/alice/notes.txt": "alice's notes"}
	for path, text := range sources {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	firstHash, _, err := first.CopyFile(parent + "/one/Users/Alice/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = second.CopyFile(parent + "/two/Users/alice/notes.txt")
	staged, readErr := os.ReadFile(parent + "/stage/fs/Users/Alice/notes.txt")
	sum := sha256.Sum256(staged)
	if readErr != nil || hex.EncodeToString(sum[:]) != firstHash {
		t.Errorf("Alice's staged copy holds %q (%v) after alice's copy; want Alice's bytes, "+
			"which her row hashed", staged, readErr)
	}
	if err == nil {
		t.Error("alice's copy was staged onto Alice's staged path without an error")
	}
}
