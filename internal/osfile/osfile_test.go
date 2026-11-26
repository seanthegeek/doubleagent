package osfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// wrapperCall is one wrapper called on a path, reduced to its error.
type wrapperCall struct {
	name string
	call func(path string) error
}

// missingParentCalls returns every wrapper that can fail on a path whose
// parent does not exist, each reduced to the error it returns.
func missingParentCalls() []wrapperCall {
	closing := func(file *os.File, err error) error {
		if err == nil {
			_ = file.Close()
		}
		return err
	}
	return []wrapperCall{
		{"Open", func(path string) error { return closing(Open(path)) }},
		{"OpenFile", func(path string) error { return closing(OpenFile(path, os.O_RDONLY, 0)) }},
		{"CreateNew", func(path string) error { return closing(CreateNew(path)) }},
		{"Lstat", func(path string) error { _, err := Lstat(path); return err }},
		{"Stat", func(path string) error { _, err := Stat(path); return err }},
		{"Readlink", func(path string) error { _, err := Readlink(path); return err }},
		{"ReadDir", func(path string) error { _, err := ReadDir(path); return err }},
		{"ReadFile", func(path string) error { _, err := ReadFile(path); return err }},
		{"WriteFile", func(path string) error { return WriteFile(path, []byte("x"), 0o644) }},
		{"Remove", Remove},
		{"Rename", func(path string) error { return Rename(path, path+"-renamed") }},
		{"Chtimes", func(path string) error { return Chtimes(path, time.Now(), time.Now()) }},
	}
}

func TestWrappersReportAMissingPathAsNotExist(t *testing.T) {
	root := t.TempDir()
	for _, wrapper := range missingParentCalls() {
		t.Run(wrapper.name+" on a path under a missing directory fails with not-exist", func(t *testing.T) {
			err := wrapper.call(filepath.Join(root, "missing", "child"))
			if !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s error %v, want one matching fs.ErrNotExist", wrapper.name, err)
			}
		})
	}
}

func TestRemoveAllOfAMissingPathSucceedsAsOsDoes(t *testing.T) {
	if err := RemoveAll(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Errorf("RemoveAll(missing) = %v, want nil", err)
	}
}

// writeText writes text to path or fails the test.
func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readText returns the contents of path, read with os, or fails the test.
func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWrappersActLikeOsOnAnOrdinaryPath(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, root string)
	}{
		{"Open reads the file", checkOpen},
		{"OpenFile with create and write makes the file", checkOpenFile},
		{"Lstat and Stat describe the file", checkStats},
		{"ReadDir lists the entries of a directory", checkReadDir},
		{"ReadFile returns the contents", checkReadFile},
		{"WriteFile replaces the contents", checkWriteFile},
		{"MkdirAll makes nested directories and accepts an existing one", checkMkdirAll},
		{"MkdirTemp makes two new directories under its pattern, in the native form", checkMkdirTemp},
		{"Remove deletes a file and refuses a directory that holds one", checkRemove},
		{"RemoveAll deletes a tree", checkRemoveAll},
		{"Rename replaces an existing file and refuses a directory", checkRename},
		{"Chtimes sets the modification time", checkChtimes},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { test.run(t, t.TempDir()) })
	}
}

// checkOpen checks that Open reads the file.
func checkOpen(t *testing.T, root string) {
	writeText(t, filepath.Join(root, "file"), "content")
	file, err := Open(filepath.Join(root, "file"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data := make([]byte, 20)
	count, _ := file.Read(data)
	if got := string(data[:count]); got != "content" {
		t.Errorf("Open read %q, want %q", got, "content")
	}
}

// checkOpenFile checks that OpenFile with create and write makes the file.
func checkOpenFile(t *testing.T, root string) {
	path := filepath.Join(root, "file")
	file, err := OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("made"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path); got != "made" {
		t.Errorf("file holds %q, want %q", got, "made")
	}
	if _, err := OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644); !errors.Is(err, fs.ErrExist) {
		t.Errorf("second exclusive create error %v, want fs.ErrExist", err)
	}
}

// checkStats checks that Lstat and Stat describe the file.
func checkStats(t *testing.T, root string) {
	writeText(t, filepath.Join(root, "file"), "12345")
	for name, stat := range map[string]func(string) (fs.FileInfo, error){"Lstat": Lstat, "Stat": Stat} {
		entry, err := stat(filepath.Join(root, "file"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if entry.Size() != 5 || !entry.Mode().IsRegular() || entry.Name() != "file" {
			t.Errorf("%s = name %q size %d mode %v, want file, 5, regular",
				name, entry.Name(), entry.Size(), entry.Mode())
		}
	}
}

// checkReadDir checks that ReadDir lists the entries of a directory.
func checkReadDir(t *testing.T, root string) {
	writeText(t, filepath.Join(root, "b"), "")
	writeText(t, filepath.Join(root, "a"), "")
	if err := os.Mkdir(filepath.Join(root, "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if strings.Join(names, ",") != "a,b,c" {
		t.Errorf("ReadDir names %q, want a, b, c", names)
	}
}

// checkReadFile checks that ReadFile returns the contents.
func checkReadFile(t *testing.T, root string) {
	writeText(t, filepath.Join(root, "file"), "content")
	data, err := ReadFile(filepath.Join(root, "file"))
	if err != nil || string(data) != "content" {
		t.Errorf("ReadFile = %q, %v, want %q", data, err, "content")
	}
}

// checkWriteFile checks that WriteFile replaces the contents.
func checkWriteFile(t *testing.T, root string) {
	path := filepath.Join(root, "file")
	writeText(t, path, "long old content")
	if err := WriteFile(path, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path); got != "new" {
		t.Errorf("file holds %q, want %q", got, "new")
	}
}

// checkMkdirAll checks that MkdirAll makes nested directories and accepts an
// existing one.
func checkMkdirAll(t *testing.T, root string) {
	nested := filepath.Join(root, "one", "two", "three")
	for attempt := 0; attempt < 2; attempt++ {
		if err := MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("MkdirAll attempt %d: %v", attempt, err)
		}
	}
	if entry, err := os.Stat(nested); err != nil || !entry.IsDir() {
		t.Errorf("nested path is not a directory: %v", err)
	}
}

// checkMkdirTemp checks that MkdirTemp makes two new directories under its
// pattern.
func checkMkdirTemp(t *testing.T, root string) {
	first, err := MkdirTemp(root, "cac-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	second, err := MkdirTemp(root, "cac-*")
	if err != nil {
		t.Fatalf("MkdirTemp again: %v", err)
	}
	for _, made := range []string{first, second} {
		if filepath.Dir(made) != root || !strings.HasPrefix(filepath.Base(made), "cac-") {
			t.Errorf("made %q, want a cac- directory directly under %q", made, root)
		}
		if entry, err := os.Stat(made); err != nil || !entry.IsDir() {
			t.Errorf("%q is not a directory: %v", made, err)
		}
	}
	if first == second {
		t.Errorf("both calls made %q", first)
	}
	if _, err := MkdirTemp(filepath.Join(root, "absent"), "cac-*"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("in a directory that does not exist: %v, want not exist", err)
	}
}

// checkRemove checks that Remove deletes a file and refuses a directory that
// holds one.
func checkRemove(t *testing.T, root string) {
	writeText(t, filepath.Join(root, "file"), "")
	if err := os.Mkdir(filepath.Join(root, "full"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(root, "full", "child"), "")
	if err := Remove(filepath.Join(root, "file")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "file")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("file still there after Remove: %v", err)
	}
	if err := Remove(filepath.Join(root, "full")); err == nil {
		t.Error("Remove of a directory holding a file succeeded, want an error")
	}
}

// checkRemoveAll checks that RemoveAll deletes a tree.
func checkRemoveAll(t *testing.T, root string) {
	if err := os.MkdirAll(filepath.Join(root, "tree", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(root, "tree", "deep", "file"), "")
	if err := RemoveAll(filepath.Join(root, "tree")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "tree")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("tree still there after RemoveAll: %v", err)
	}
}

// checkRename checks that Rename moves a file over an existing one, which
// then holds the moved content, and fails onto a directory, leaving both.
func checkRename(t *testing.T, root string) {
	writeText(t, filepath.Join(root, "new"), "new")
	writeText(t, filepath.Join(root, "old"), "old and longer")
	if err := Rename(filepath.Join(root, "new"), filepath.Join(root, "old")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "new")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the moved name is still there after Rename: %v", err)
	}
	if text := readText(t, filepath.Join(root, "old")); text != "new" {
		t.Errorf("the replaced file holds %q, want %q", text, "new")
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Rename(filepath.Join(root, "old"), filepath.Join(root, "directory")); err == nil {
		t.Error("Rename onto a directory succeeded, want an error")
	}
	if text := readText(t, filepath.Join(root, "old")); text != "new" {
		t.Errorf("the file Rename failed to move holds %q, want %q", text, "new")
	}
}

// checkChtimes checks that Chtimes sets the modification time.
func checkChtimes(t *testing.T, root string) {
	path := filepath.Join(root, "file")
	writeText(t, path, "")
	moment := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := Chtimes(path, moment, moment); err != nil {
		t.Fatal(err)
	}
	entry, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !entry.ModTime().Equal(moment) {
		t.Errorf("modification time %v, want %v", entry.ModTime(), moment)
	}
}

func TestReadlinkAndLstatSeeALinkAndStatFollowsIt(t *testing.T) {
	root := t.TempDir()
	writeText(t, filepath.Join(root, "target"), "12345")
	if err := os.Symlink("target", filepath.Join(root, "link")); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	link := filepath.Join(root, "link")
	if got, err := Readlink(link); err != nil || got != "target" {
		t.Errorf("Readlink = %q, %v, want %q", got, err, "target")
	}
	if entry, err := Lstat(link); err != nil || entry.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("Lstat of a link: mode %v, err %v, want a symlink", entry, err)
	}
	if entry, err := Stat(link); err != nil || entry.Size() != 5 {
		t.Errorf("Stat of a link: %v, %v, want the target's size 5", entry, err)
	}
}

func TestCreateNewRefusesAFileOrADanglingLinkAtTheName(t *testing.T) {
	root := t.TempDir()
	fresh := filepath.Join(root, "fresh")
	file, err := CreateNew(fresh)
	if err != nil {
		t.Fatalf("CreateNew of a free name: %v", err)
	}
	_ = file.Close()
	if file, err := CreateNew(fresh); !errors.Is(err, fs.ErrExist) {
		if err == nil {
			_ = file.Close()
		}
		t.Errorf("CreateNew of an existing file: %v, want an exists error", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "missing"), link); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	if file, err := CreateNew(link); !errors.Is(err, fs.ErrExist) {
		if err == nil {
			_ = file.Close()
		}
		t.Errorf("CreateNew of a dangling link: %v, want an exists error", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "missing")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the link's target was created: %v", err)
	}
}
