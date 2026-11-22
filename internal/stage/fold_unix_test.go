//go:build darwin || linux

package stage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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
	if !mustTree(t, parent+"/stage/fs").FoldsCase(".") {
		t.Fatal("FoldsCase says the staging directory tells case apart")
	}
	first := Stager{Dir: parent + "/stage", Tree: mustTree(t, parent+"/stage"), Root: parent + "/one",
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
	firstHash, _, err := copyFile(first, parent+"/one/Users/Alice/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = copyFile(second, parent+"/two/Users/alice/notes.txt")
	staged, readErr := os.ReadFile(parent + "/stage/fs/Users/Alice/notes.txt")
	sum := sha256.Sum256(staged)
	if readErr != nil || hex.EncodeToString(sum[:]) != firstHash {
		t.Errorf("Alice's staged copy holds %q (%v) after alice's copy; want Alice's bytes, "+
			"which her row hashed", staged, readErr)
	}
	if !errors.As(err, &collision{}) {
		t.Errorf("alice's copy gave %v; want the collision error", err)
	}
	if !first.FailedCopyInside(parent+"/stage/fs/Users/alice/notes.txt") ||
		!fileHolds(parent+"/stage/fs/Users/Alice/notes.txt", "Alice's notes") {
		t.Error("cleaning up alice's refused copy touched Alice's")
	}
}

func TestALinkIsNotStagedOverAnotherEntrysLinkInAFoldingStage(t *testing.T) {
	parent := foldingStage(t)
	first := Stager{Dir: parent + "/stage", Tree: mustTree(t, parent+"/stage"), Root: parent + "/one",
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
	if err := symlink(first, parent+"/one/Users/Alice/link", "target", false); err != nil {
		t.Fatal(err)
	}
	if err := symlink(first, parent+"/one/Users/Alice/link", "target", false); err != nil {
		t.Errorf("staging Alice's link again = %v; want it left in place", err)
	}
	if err := symlink(second, parent+"/two/Users/alice/link", "target", false); !errors.As(err,
		&collision{}) {
		t.Errorf("staging alice's link over Alice's = %v; want the collision error", err)
	}
}

func TestOneSourceReachedThroughTwoSpellingsIsStagedTwice(t *testing.T) {
	// On a volume that folds case, the catalog's AppData/Roaming/Devin and
	// AppData/Roaming/devin name one directory; both rows are collected.
	parent := foldingStage(t)
	stager := Stager{Dir: parent + "/stage", Tree: mustTree(t, parent+"/stage"), Root: parent + "/root",
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
		if _, _, err := copyFile(stager, spelling); err != nil {
			t.Errorf("Copy(%s) = %v; want the one source file staged again", spelling, err)
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
	_, caseErr := os.Lstat(dir + "/X")
	if err := os.WriteFile(dir+"/"+composed, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, formErr := os.Lstat(dir + "/" + decomposed)
	if err := os.Remove(dir + "/" + composed); err != nil {
		t.Fatal(err)
	}
	merges := caseErr == nil || formErr == nil
	if got := mustTree(t, dir).FoldsCase("."); got != merges {
		t.Errorf("FoldsCase = %v; the directory takes letter case or Unicode forms for one: %v",
			got, merges)
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
	stager := Stager{Dir: parent + "/stage", Tree: mustTree(t, parent+"/stage"), Root: parent + "/root",
		FoldsCase: true, Places: NewPlaces()}
	source := parent + "/root/x"
	if err := os.MkdirAll(parent+"/root", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := copyFile(stager, source); err != nil {
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
	if _, _, err := copyFile(stager, parent+"/root/X"); err != nil {
		t.Errorf("Copy of the same open file under another spelling = %v; want it staged "+
			"again, judged by the file it opened, not by what the path names later", err)
	}
}

// twoFoldingStagers returns two stagers over the roots one and two under
// parent, staging into one folding staging directory with one record.
func twoFoldingStagers(t *testing.T, parent string) (Stager, Stager) {
	first := Stager{Dir: parent + "/stage", Tree: mustTree(t, parent+"/stage"), Root: parent + "/one",
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
			first, second := twoFoldingStagers(t, parent)
			stage := func(stager Stager, path string, link bool) error {
				makeSource(t, path, link)
				if link {
					if err := stager.MakeParent(path); err != nil {
						return err
					}
					return symlink(stager, path, "t", false)
				}
				_, _, err := copyFile(stager, path)
				return err
			}
			if err := stage(first, parent+"/one/"+testCase.first, testCase.firstIsLink); err != nil {
				t.Fatal(err)
			}
			err := stage(second, parent+"/two/"+testCase.second, testCase.secondIsLink)
			if !IsCollision(err) {
				t.Errorf("staging %s after %s = %v; want the collision error", testCase.second,
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

func TestAnEarlierRespelledComponentIsACaseCollision(t *testing.T) {
	t.Run("a copy under another entry's link reached through another case", func(t *testing.T) {
		parent := foldingStage(t)
		first, second := twoFoldingStagers(t, parent)
		link := parent + "/one/a/cursor"
		makeSource(t, link, true)
		if err := first.MakeParent(link); err != nil {
			t.Fatal(err)
		}
		if err := symlink(first, link, "t", true); err != nil {
			t.Fatal(err)
		}
		file := parent + "/two/A/cursor/User/x"
		makeSource(t, file, false)
		if _, _, err := copyFile(second, file); !IsCollision(err) {
			t.Errorf("Copy of A/cursor/User/x = %v; want the collision error", err)
		}
	})
	t.Run("a file where another entry's directory sits in another case", func(t *testing.T) {
		parent := foldingStage(t)
		first, second := twoFoldingStagers(t, parent)
		makeSource(t, parent+"/one/Zed/two/x", false)
		if _, _, err := copyFile(first, parent+"/one/Zed/two/x"); err != nil {
			t.Fatal(err)
		}
		makeSource(t, parent+"/two/zed/two", false)
		if _, _, err := copyFile(second, parent+"/two/zed/two"); !IsCollision(err) {
			t.Errorf("Copy of zed/two = %v; want the collision error", err)
		}
	})
}

// failingWriter fails every write.
type failingWriter struct{}

// Write fails.
func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("file too large") }

func TestAFailedCopyInADirectorySpelledInAnotherCaseIsRemoved(t *testing.T) {
	parent := foldingStage(t)
	first, second := twoFoldingStagers(t, parent)
	makeSource(t, parent+"/one/Zed/one", false)
	if _, _, err := copyFile(first, parent+"/one/Zed/one"); err != nil {
		t.Fatal(err)
	}
	second.wrapWriter = func(io.Writer) io.Writer { return failingWriter{} }
	makeSource(t, parent+"/two/zed/two", false)
	_, dest, _ := second.Dest(parent + "/two/zed/two")
	if _, _, err := copyFile(second, parent+"/two/zed/two"); err == nil {
		t.Fatal("the copy did not fail")
	}
	if !second.FailedCopyInside(dest) {
		t.Error("FailedCopyInside = false; the staged path lies inside the staging directory")
	}
	if _, err := os.Lstat(parent + "/stage/fs/Zed/two"); err == nil {
		t.Error("the partial copy of zed/two is left in Zed, an archive member with no row")
	}
	if len(second.Places.staged) != 1 {
		t.Errorf("the record holds %d staged files; want Zed/one's alone, the failed copy's "+
			"record gone with its file", len(second.Places.staged))
	}
	if !fileHolds(parent+"/stage/fs/Zed/one", parent+"/one/Zed/one") {
		t.Error("Zed/one's copy was touched")
	}
}

func TestTheSameSourceStagedAgainKeepsTheFirstCopy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes a read-only copy")
	}
	stageTwice := func(t *testing.T, stager Stager, first, second string) (Copied, Copied, int) {
		t.Helper()
		writes := 0
		stager.wrapWriter = func(target io.Writer) io.Writer { writes++; return target }
		one, err := stager.Copy(first)
		if err != nil {
			t.Fatal(err)
		}
		// Any second write fails, as a read error or a full disk would.
		stager.wrapWriter = func(io.Writer) io.Writer { writes++; return failingWriter{} }
		two, err := stager.Copy(second)
		if err != nil {
			t.Errorf("staging %s again = %v; want the first copy's member", second, err)
		}
		return one, two, writes
	}
	for _, readOnly := range []bool{false, true} {
		name := fmt.Sprintf("through another spelling on a stage that folds case, read-only %v",
			readOnly)
		t.Run(name, func(t *testing.T) {
			parent := foldingStage(t)
			stager := Stager{Dir: parent + "/stage", Tree: mustTree(t, parent+"/stage"),
				Root: parent + "/root", FoldsCase: true, Places: NewPlaces()}
			makeSource(t, parent+"/root/Devin/x", false)
			if readOnly {
				if err := os.Chmod(parent+"/root/Devin/x", 0o444); err != nil {
					t.Fatal(err)
				}
			}
			one, two, writes := stageTwice(t, stager, parent+"/root/Devin/x",
				parent+"/root/devin/x")
			if two != one || writes != 1 || !fileHolds(parent+"/stage/"+one.ArchivePath,
				parent+"/root/Devin/x") {
				t.Errorf("second copy %+v after %+v with %d writes; want the first copy's "+
					"member, written once and intact", two, one, writes)
			}
		})
	}
	t.Run("by the same path on a stage that tells case apart", func(t *testing.T) {
		stager, root := newStager(t)
		stager.Places = NewPlaces()
		makeSource(t, root+"/home/x", false)
		if err := os.Chmod(root+"/home/x", 0o444); err != nil {
			t.Fatal(err)
		}
		one, two, writes := stageTwice(t, stager, root+"/home/x", root+"/home/x")
		if two != one || writes != 1 || !fileHolds(stager.Dir+"/"+one.ArchivePath, root+"/home/x") {
			t.Errorf("second copy %+v after %+v with %d writes; want the first copy's member, "+
				"written once and intact", two, one, writes)
		}
	})
}
