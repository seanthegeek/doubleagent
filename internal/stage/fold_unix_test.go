//go:build darwin || linux

package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	if !FoldsCase(parent + "/stage/fs") {
		t.Fatal("FoldsCase says the staging directory tells case apart")
	}
	first := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/one",
		FoldsCase: true}
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
	if !errors.As(err, &caseCollision{}) {
		t.Errorf("alice's copy gave %v; want the case collision error", err)
	}
	if !first.RemoveFailedCopy(parent+"/stage/fs/Users/alice/notes.txt") ||
		!fileHolds(parent+"/stage/fs/Users/Alice/notes.txt", "Alice's notes") {
		t.Error("cleaning up alice's refused copy touched Alice's")
	}
}

func TestNoLinkLandsInAnotherEntrysDirectoryInAFoldingStage(t *testing.T) {
	parent := foldingStage(t)
	stager := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/one",
		FoldsCase: true}
	if err := os.MkdirAll(parent+"/stage/fs/Users/Alice", 0o700); err != nil {
		t.Fatal(err)
	}
	err := stager.Symlink(parent+"/one/Users/alice/link", "target", false)
	if !errors.As(err, &caseCollision{}) {
		t.Errorf("Symlink = %v; want the case collision error", err)
	}
	if _, err := os.Lstat(parent + "/stage/fs/Users/Alice/link"); err == nil {
		t.Error("alice's link was left in Alice's staged directory")
	}
}

func TestFoldsCaseAnswersForTheStagingDirectory(t *testing.T) {
	dir := t.TempDir()
	probe := dir + "/x"
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Lstat(dir + "/X")
	if got := FoldsCase(dir); got != (err == nil) {
		t.Errorf("FoldsCase = %v; the directory folds case: %v", got, err == nil)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("FoldsCase left %d entries, want only the test's own", len(entries))
	}
}

// fileHolds reports whether the file at path holds text.
func fileHolds(path, text string) bool {
	content, err := os.ReadFile(path)
	return err == nil && string(content) == text
}
