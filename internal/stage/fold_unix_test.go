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
		FoldsCase: true, Places: NewPlaces()}
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

func TestALinkIsNotStagedOverAnotherEntrysLinkInAFoldingStage(t *testing.T) {
	parent := foldingStage(t)
	first := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/one",
		FoldsCase: true, Places: NewPlaces()}
	second := first
	second.Root = parent + "/two"
	for _, link := range []string{parent + "/one/Users/Alice/link", parent + "/two/Users/alice/link"} {
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", link); err != nil {
			t.Fatal(err)
		}
	}
	if err := first.MakeParent(parent + "/one/Users/Alice/link"); err != nil {
		t.Fatal(err)
	}
	if err := first.Symlink(parent+"/one/Users/Alice/link", "target", false); err != nil {
		t.Fatal(err)
	}
	if err := first.Symlink(parent+"/one/Users/Alice/link", "target", false); err != nil {
		t.Errorf("staging Alice's link again = %v; want it left in place", err)
	}
	if err := second.Symlink(parent+"/two/Users/alice/link", "target", false); !errors.As(err,
		&caseCollision{}) {
		t.Errorf("staging alice's link over Alice's = %v; want the case collision error", err)
	}
}

func TestOneSourceReachedThroughTwoSpellingsIsStagedTwice(t *testing.T) {
	// On a volume that folds case, the catalog's AppData/Roaming/Devin and
	// AppData/Roaming/devin name one directory; both rows are collected.
	parent := foldingStage(t)
	stager := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/root",
		FoldsCase: true, Places: NewPlaces()}
	source := parent + "/root/Roaming/devin/User/settings.json"
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{source, parent + "/root/Roaming/Devin/User/settings.json",
		parent + "/root/Roaming/Devin/User/settings.json"} {
		if _, _, err := stager.CopyFile(spelling); err != nil {
			t.Errorf("CopyFile(%s) = %v; want the one source file staged again", spelling, err)
		}
	}
	if !fileHolds(parent+"/stage/fs/Roaming/devin/User/settings.json", "{}\n") {
		t.Error("the staged copy does not hold the source's bytes")
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

func TestTheRecordNamesTheFileWhoseBytesWereCopied(t *testing.T) {
	parent := foldingStage(t)
	stager := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/root",
		FoldsCase: true, Places: NewPlaces()}
	source := parent + "/root/x"
	if err := os.MkdirAll(parent+"/root", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := stager.CopyFile(source); err != nil {
		t.Fatal(err)
	}
	// The second spelling opens the same file; then the path is swapped for
	// another file before anything else looks at it.
	stager.afterOpen = func(string) {
		if err := os.Rename(source, parent+"/root/moved"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, []byte("swapped in"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := stager.CopyFile(parent + "/root/X"); err != nil {
		t.Errorf("CopyFile of the same open file under another spelling = %v; want it staged "+
			"again, judged by the file it opened, not by what the path names later", err)
	}
}

func TestANonFoldingStageChecksNoLinkPlace(t *testing.T) {
	stager, root := newStager(t)
	stager.Places = NewPlaces()
	if err := os.Symlink("target", root+"/home/link"); err != nil {
		t.Fatal(err)
	}
	if err := stager.MakeParent(root + "/home/link"); err != nil {
		t.Fatal(err)
	}
	if err := stager.Symlink(root+"/home/link", "target", false); err != nil {
		t.Fatal(err)
	}
	if len(stager.Places.sources) != 0 {
		t.Errorf("a stage that tells case apart recorded %d link places, each an extra lstat; "+
			"want none", len(stager.Places.sources))
	}
}

// twoFoldingStagers returns two stagers over the roots one and two under
// parent, staging into one folding staging directory with one record.
func twoFoldingStagers(parent string) (Stager, Stager) {
	first := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/one",
		FoldsCase: true, Places: NewPlaces()}
	second := first
	second.Root = parent + "/two"
	return first, second
}

// makeSource makes path, a regular file holding its name, or with link set
// a symlink to t, with its parents.
func makeSource(t *testing.T, path string, link bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var err error
	if link {
		err = os.Symlink("t", path)
	} else {
		err = os.WriteFile(path, []byte(path), 0o644)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestAFileWhosePlaceHoldsALinkOrDirectoryIsACaseCollision(t *testing.T) {
	cases := []struct {
		name          string
		first, second string
		firstIsLink   bool
		secondIsLink  bool
	}{
		{"a file after a link of another case", "P", "p", true, false},
		{"a link after a file of another case", "Q", "q", false, true},
		{"a file after a directory of another case", "R/x", "r", false, false},
		{"a directory after a file of another case", "s", "S/x", false, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			parent := foldingStage(t)
			first, second := twoFoldingStagers(parent)
			stage := func(stager Stager, path string, link bool) error {
				makeSource(t, path, link)
				if link {
					if err := stager.MakeParent(path); err != nil {
						return err
					}
					return stager.Symlink(path, "t", false)
				}
				_, _, err := stager.CopyFile(path)
				return err
			}
			if err := stage(first, parent+"/one/"+testCase.first, testCase.firstIsLink); err != nil {
				t.Fatal(err)
			}
			err := stage(second, parent+"/two/"+testCase.second, testCase.secondIsLink)
			if !IsCaseCollision(err) {
				t.Errorf("staging %s after %s = %v; want the case collision error", testCase.second,
					testCase.first, err)
			}
		})
	}
}

func TestACopyAndALinkOfOneFileHaveOneIdentity(t *testing.T) {
	path := t.TempDir() + "/file"
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fromHandle, fromLstat := handleIdentity(file, info), linkIdentity(path); fromHandle !=
		fromLstat || fromHandle == [2]uint64{} {
		t.Errorf("identity from the open file %v, from lstat %v; want one non-zero form",
			fromHandle, fromLstat)
	}
}
