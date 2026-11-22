//go:build darwin || linux

package stage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Composed and decomposed spellings of one name: é as one character, and as
// e followed by a combining acute accent.
const (
	composed   = "café"
	decomposed = "café"
)

// volumes returns how each test gets the directory its stages live in: the
// temporary directory's volume, and a case-sensitive APFS volume, which
// keeps letter case apart but takes the two Unicode forms of a name for one.
func volumes() map[string]func(t *testing.T) string {
	return map[string]func(t *testing.T) string{
		"on the temporary directory's volume": func(t *testing.T) string {
			parent, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			return parent
		},
		"on a case-sensitive APFS volume": caseSensitiveVolume,
	}
}

// unicodeStagers returns two stagers over the roots one and two under
// parent, sharing one staging directory and one record, with FoldsCase as
// the run's probe gives it; the test is skipped where the staging directory
// keeps the two Unicode forms of a name apart.
func unicodeStagers(t *testing.T, parent string) (Stager, Stager) {
	t.Helper()
	fsDir := parent + "/stage/fs"
	if err := os.MkdirAll(fsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fsDir+"/"+composed, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := os.Lstat(fsDir + "/" + decomposed)
	if removeErr := os.Remove(fsDir + "/" + composed); removeErr != nil {
		t.Fatal(removeErr)
	}
	if err != nil {
		t.Skipf("%s keeps the two Unicode forms of a name apart", fsDir)
	}
	first := Stager{Dir: parent + "/stage", Physical: parent + "/stage", Root: parent + "/one",
		FoldsCase: FoldsCase(fsDir), Places: NewPlaces()}
	second := first
	second.Root = parent + "/two"
	return first, second
}

// writeSource makes path, a regular file holding text, with its parents.
func writeSource(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTwoFilesWhoseNamesDifferInUnicodeFormAreNotStagedOverEachOther(t *testing.T) {
	for name, volume := range volumes() {
		t.Run(name, func(t *testing.T) {
			parent := volume(t)
			first, second := unicodeStagers(t, parent)
			writeSource(t, parent+"/one/home/"+composed+"/notes", "first entry")
			writeSource(t, parent+"/two/home/"+decomposed+"/notes", "second entry")
			one, err := first.Copy(parent + "/one/home/" + composed + "/notes")
			if err != nil {
				t.Fatal(err)
			}
			_, err = second.Copy(parent + "/two/home/" + decomposed + "/notes")
			if !IsCollision(err) {
				t.Errorf("the second entry's copy = %v; want it refused as a collision", err)
			}
			if !fileHolds(parent+"/stage/"+one.ArchivePath, "first entry") {
				t.Errorf("the first entry's member %s no longer holds its bytes", one.ArchivePath)
			}
		})
	}
}

func TestOneSourceReachedThroughTwoUnicodeFormsIsWrittenOnce(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes a read-only copy")
	}
	for name, volume := range volumes() {
		for _, readOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, source read-only %v", name, readOnly), func(t *testing.T) {
				parent := volume(t)
				stager, _ := unicodeStagers(t, parent)
				writeSource(t, parent+"/one/"+composed, "the one source")
				if readOnly {
					if err := os.Chmod(parent+"/one/"+composed, 0o444); err != nil {
						t.Fatal(err)
					}
				}
				writes := 0
				stager.wrapWriter = func(target io.Writer) io.Writer { writes++; return target }
				one, err := stager.Copy(parent + "/one/" + composed)
				if err != nil {
					t.Fatal(err)
				}
				// A second write fails, as a read error or a full disk would.
				stager.wrapWriter = func(io.Writer) io.Writer { writes++; return failingWriter{} }
				two, err := stager.Copy(parent + "/one/" + decomposed)
				if err != nil || two != one || writes != 1 {
					t.Errorf("the second spelling = %+v, %v after %+v, %d writes; want the first "+
						"result, written once", two, err, one, writes)
				}
				if !fileHolds(parent+"/stage/"+one.ArchivePath, "the one source") {
					t.Errorf("the member %s no longer holds the source's bytes", one.ArchivePath)
				}
			})
		}
	}
}

func TestALinkIsNotStagedOverAnotherEntryInAnotherUnicodeForm(t *testing.T) {
	cases := []struct {
		name  string
		first func(t *testing.T, stager Stager, parent string) error
	}{
		{"over another link", func(t *testing.T, stager Stager, parent string) error {
			path := parent + "/one/" + composed
			if err := os.MkdirAll(parent+"/one", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("first", path); err != nil {
				t.Fatal(err)
			}
			_, err := stager.Link(path, "first", false)
			return err
		}},
		{"over a file", func(t *testing.T, stager Stager, parent string) error {
			writeSource(t, parent+"/one/"+composed, "a file")
			_, err := stager.Copy(parent + "/one/" + composed)
			return err
		}},
		{"over a directory", func(t *testing.T, stager Stager, parent string) error {
			writeSource(t, parent+"/one/"+composed+"/x", "a file in a directory")
			_, err := stager.Copy(parent + "/one/" + composed + "/x")
			return err
		}},
	}
	for name, volume := range volumes() {
		for _, testCase := range cases {
			t.Run(name+", "+testCase.name, func(t *testing.T) {
				parent := volume(t)
				first, second := unicodeStagers(t, parent)
				if err := testCase.first(t, first, parent); err != nil {
					t.Fatal(err)
				}
				link := parent + "/two/" + decomposed
				if err := os.MkdirAll(parent+"/two", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("second", link); err != nil {
					t.Fatal(err)
				}
				if _, err := second.Link(link, "second", false); !IsCollision(err) {
					t.Errorf("the second entry's link = %v; want it refused as a collision", err)
				}
			})
		}
	}
}

func TestTheProbeSeesNamesTheVolumeTakesForOne(t *testing.T) {
	for name, volume := range volumes() {
		t.Run(name, func(t *testing.T) {
			dir := volume(t)
			if err := os.WriteFile(dir+"/"+composed, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			_, merges := os.Lstat(dir + "/" + decomposed)
			if err := os.Remove(dir + "/" + composed); err != nil {
				t.Fatal(err)
			}
			if merges == nil && !FoldsCase(dir) {
				t.Errorf("FoldsCase(%s) = false; the volume takes %q and %q for one", dir,
					composed, decomposed)
			}
		})
	}
}
