package osfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// extended returns the \\?\ form of root joined with name, the form the
// tests build their fixtures through so Windows keeps a trailing dot or space.
func extended(root, name string) string {
	return `\\?\` + filepath.Join(root, name)
}

// makeEntries creates the entries named, through \\?\ paths: a name ending in
// a slash is a directory, any other a file holding the text after the "=".
func makeEntries(t *testing.T, root string, entries ...string) {
	t.Helper()
	for _, entry := range entries {
		name, text, isFile := strings.Cut(entry, "=")
		path := extended(root, name)
		if !isFile {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// textAt returns the contents of the entry name under root, read through the
// \\?\ form, or fails the test.
func textAt(t *testing.T, root, name string) string {
	t.Helper()
	data, err := os.ReadFile(extended(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// existsAt reports whether the entry name under root exists.
func existsAt(root, name string) bool {
	_, err := os.Lstat(extended(root, name))
	return err == nil
}

// neighbours returns a directory holding x. and x (each with its own entry)
// and the files "y " and y, each with its own content.
func neighbours(t *testing.T) string {
	root := t.TempDir()
	makeEntries(t, root, "x.", "x.\\dotted-entry=dot", "x", "x\\plain-entry=plain",
		"y =space-content", "y=plain")
	return root
}

func TestWrappersReachTheEntryNamedNotItsNeighbour(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, root string)
	}{
		{"Open reads the file with a trailing space", checkOpenSpace},
		{"OpenFile appends to the file with a trailing space only", checkOpenFileSpace},
		{"Create makes the file with a trailing dot beside its neighbour", checkCreateDot},
		{"Lstat and Stat give the size of the file named", checkStatSizes},
		{"Readlink reads the link with a trailing dot", checkReadlinkDot},
		{"ReadDir lists the directory with a trailing dot", checkReadDirDot},
		{"ReadFile reads the file with a trailing space", checkReadFileSpace},
		{"WriteFile writes the file with a trailing dot beside its neighbour", checkWriteFileDot},
		{"MkdirAll makes a child inside the directory with a trailing dot", checkMkdirAllDot},
		{"Remove deletes the directory with a trailing dot and keeps its neighbour", checkRemoveDot},
		{"RemoveAll deletes the tree with a trailing dot and keeps its neighbour", checkRemoveAllDot},
		{"Chtimes dates the file with a trailing space and not its neighbour", checkChtimesSpace},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { test.run(t, neighbours(t)) })
	}
}

// checkOpenSpace checks that Open reads the file with a trailing space.
func checkOpenSpace(t *testing.T, root string) {
	file, err := Open(filepath.Join(root, "y "))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	data := make([]byte, 40)
	count, _ := file.Read(data)
	if got := string(data[:count]); got != "space-content" {
		t.Errorf("Open(y with space) read %q, want %q", got, "space-content")
	}
}

// checkOpenFileSpace checks that OpenFile appends to the file with a
// trailing space only.
func checkOpenFileSpace(t *testing.T, root string) {
	file, err := OpenFile(filepath.Join(root, "y "), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("+more"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if got := textAt(t, root, "y "); got != "space-content+more" {
		t.Errorf("file with space holds %q", got)
	}
	if got := textAt(t, root, "y"); got != "plain" {
		t.Errorf("neighbour y holds %q, want it unchanged", got)
	}
}

// checkCreateDot checks that Create makes the file with a trailing dot
// beside its neighbour.
func checkCreateDot(t *testing.T, root string) {
	makeEntries(t, root, "z=keep")
	file, err := Create(filepath.Join(root, "z."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if got := textAt(t, root, "z."); got != "new" {
		t.Errorf("z. holds %q, want %q", got, "new")
	}
	if got := textAt(t, root, "z"); got != "keep" {
		t.Errorf("neighbour z holds %q, want it unchanged", got)
	}
}

// checkStatSizes checks that Lstat and Stat give the size of the file named.
func checkStatSizes(t *testing.T, root string) {
	for name, stat := range map[string]func(string) (fs.FileInfo, error){"Lstat": Lstat, "Stat": Stat} {
		spaced, err := stat(filepath.Join(root, "y "))
		if err != nil || spaced.Size() != int64(len("space-content")) {
			t.Errorf("%s(y with space) = %v, %v, want size %d", name, spaced, err, len("space-content"))
		}
		plain, err := stat(filepath.Join(root, "y"))
		if err != nil || plain.Size() != int64(len("plain")) {
			t.Errorf("%s(y) = %v, %v, want size %d", name, plain, err, len("plain"))
		}
	}
}

// checkReadlinkDot checks that Readlink reads the link with a trailing dot.
func checkReadlinkDot(t *testing.T, root string) {
	if err := os.Symlink("dotted-target", extended(root, "l.")); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	if err := os.Symlink("plain-target", extended(root, "l")); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"l.": "dotted-target", "l": "plain-target"} {
		if got, err := Readlink(filepath.Join(root, name)); err != nil || got != want {
			t.Errorf("Readlink(%s) = %q, %v, want %q", name, got, err, want)
		}
	}
}

// checkReadDirDot checks that ReadDir lists the directory with a trailing
// dot.
func checkReadDirDot(t *testing.T, root string) {
	for name, want := range map[string]string{"x.": "dotted-entry", "x": "plain-entry"} {
		entries, err := ReadDir(filepath.Join(root, name))
		if err != nil || len(entries) != 1 || entries[0].Name() != want {
			t.Errorf("ReadDir(%s) = %v, %v, want just %s", name, entries, err, want)
		}
	}
}

// checkReadFileSpace checks that ReadFile reads the file with a trailing
// space.
func checkReadFileSpace(t *testing.T, root string) {
	if data, err := ReadFile(filepath.Join(root, "y ")); err != nil || string(data) != "space-content" {
		t.Errorf("ReadFile(y with space) = %q, %v", data, err)
	}
	if data, err := ReadFile(filepath.Join(root, "y")); err != nil || string(data) != "plain" {
		t.Errorf("ReadFile(y) = %q, %v", data, err)
	}
}

// checkWriteFileDot checks that WriteFile writes the file with a trailing
// dot beside its neighbour.
func checkWriteFileDot(t *testing.T, root string) {
	makeEntries(t, root, "v=keep")
	if err := WriteFile(filepath.Join(root, "v."), []byte("dot"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := textAt(t, root, "v."); got != "dot" {
		t.Errorf("v. holds %q, want %q", got, "dot")
	}
	if got := textAt(t, root, "v"); got != "keep" {
		t.Errorf("neighbour v holds %q, want it unchanged", got)
	}
}

// checkMkdirAllDot checks that MkdirAll makes a child inside the directory
// with a trailing dot.
func checkMkdirAllDot(t *testing.T, root string) {
	if err := MkdirAll(filepath.Join(root, "x.", "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !existsAt(root, `x.\sub\deep`) || existsAt(root, `x\sub`) {
		t.Errorf("MkdirAll made x.\\sub\\deep: %v, and x\\sub: %v", existsAt(root, `x.\sub\deep`),
			existsAt(root, `x\sub`))
	}
}

// checkRemoveDot checks that Remove deletes the entry with a trailing dot
// and keeps its neighbour.
func checkRemoveDot(t *testing.T, root string) {
	makeEntries(t, root, "r.", "r")
	if err := Remove(filepath.Join(root, "r.")); err != nil {
		t.Fatal(err)
	}
	if existsAt(root, "r.") || !existsAt(root, "r") {
		t.Errorf("after Remove(r.): r. exists %v, r exists %v, want false and true",
			existsAt(root, "r."), existsAt(root, "r"))
	}
}

// checkRemoveAllDot checks that RemoveAll deletes the tree with a trailing
// dot and keeps its neighbour.
func checkRemoveAllDot(t *testing.T, root string) {
	if err := RemoveAll(filepath.Join(root, "x.")); err != nil {
		t.Fatal(err)
	}
	if existsAt(root, "x.") || textAt(t, root, `x\plain-entry`) != "plain" {
		t.Errorf("after RemoveAll(x.): x. exists %v, want false and x\\plain-entry intact",
			existsAt(root, "x."))
	}
}

// checkChtimesSpace checks that Chtimes dates the file with a trailing space
// and not its neighbour.
func checkChtimesSpace(t *testing.T, root string) {
	before, err := os.Stat(extended(root, "y"))
	if err != nil {
		t.Fatal(err)
	}
	moment := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := Chtimes(filepath.Join(root, "y "), moment, moment); err != nil {
		t.Fatal(err)
	}
	spaced, _ := os.Stat(extended(root, "y "))
	plain, _ := os.Stat(extended(root, "y"))
	if !spaced.ModTime().Equal(moment) || !plain.ModTime().Equal(before.ModTime()) {
		t.Errorf("times: y with space %v (want %v), y %v (want unchanged %v)",
			spaced.ModTime(), moment, plain.ModTime(), before.ModTime())
	}
}

// longDirectory makes, through MkdirAll, a directory whose path is over 260
// characters, and returns it.
func longDirectory(t *testing.T) string {
	path := t.TempDir()
	for _, part := range []string{"a", "b", "c", "d", "e"} {
		path = filepath.Join(path, strings.Repeat(part, 60))
	}
	if err := MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(path) <= 260 {
		t.Fatalf("path is %d characters, want more than 260", len(path))
	}
	return path
}

func TestWrappersWorkOnAPathLongerThanTheOrdinaryLimit(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T, long string)
	}{
		{"Create, WriteFile and ReadFile make and read a file", checkLongFiles},
		{"Open and OpenFile open it", checkLongOpens},
		{"Lstat, Stat and ReadDir describe it", checkLongDescribes},
		{"Chtimes dates it", checkLongChtimes},
		{"Remove and RemoveAll delete it", checkLongRemoves},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) { test.run(t, longDirectory(t)) })
	}
}

// checkLongFiles checks that Create, WriteFile and ReadFile make and read a
// file under a long path.
func checkLongFiles(t *testing.T, long string) {
	file, err := Create(filepath.Join(long, "created"))
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if err := WriteFile(filepath.Join(long, "written"), []byte("long"), 0o644); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadFile(filepath.Join(long, "written")); err != nil || string(data) != "long" {
		t.Errorf("ReadFile = %q, %v, want %q", data, err, "long")
	}
}

// checkLongOpens checks that Open and OpenFile open a file under a long
// path.
func checkLongOpens(t *testing.T, long string) {
	if err := WriteFile(filepath.Join(long, "file"), []byte("long"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, open := range []func() (*os.File, error){
		func() (*os.File, error) { return Open(filepath.Join(long, "file")) },
		func() (*os.File, error) { return OpenFile(filepath.Join(long, "file"), os.O_RDONLY, 0) },
	} {
		file, err := open()
		if err != nil {
			t.Fatal(err)
		}
		_ = file.Close()
	}
}

// checkLongDescribes checks that Lstat, Stat and ReadDir describe a file
// under a long path.
func checkLongDescribes(t *testing.T, long string) {
	if err := WriteFile(filepath.Join(long, "file"), []byte("long"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, stat := range map[string]func(string) (fs.FileInfo, error){"Lstat": Lstat, "Stat": Stat} {
		if entry, err := stat(filepath.Join(long, "file")); err != nil || entry.Size() != 4 {
			t.Errorf("%s = %v, %v, want size 4", name, entry, err)
		}
	}
	entries, err := ReadDir(long)
	if err != nil || len(entries) != 1 || entries[0].Name() != "file" {
		t.Errorf("ReadDir = %v, %v, want just file", entries, err)
	}
}

// checkLongChtimes checks that Chtimes dates a file under a long path.
func checkLongChtimes(t *testing.T, long string) {
	if err := WriteFile(filepath.Join(long, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	moment := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	if err := Chtimes(filepath.Join(long, "file"), moment, moment); err != nil {
		t.Fatal(err)
	}
	if entry, err := Stat(filepath.Join(long, "file")); err != nil || !entry.ModTime().Equal(moment) {
		t.Errorf("Stat after Chtimes = %v, %v, want modification time %v", entry, err, moment)
	}
}

// checkLongRemoves checks that Remove and RemoveAll delete a file and a tree
// under a long path.
func checkLongRemoves(t *testing.T, long string) {
	if err := WriteFile(filepath.Join(long, "file"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Remove(filepath.Join(long, "file")); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAll(long); err != nil {
		t.Fatal(err)
	}
	if _, err := Lstat(long); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("long directory still there after RemoveAll: %v", err)
	}
}

func TestLongPathReadlinkWorks(t *testing.T) {
	long := longDirectory(t)
	if err := os.Symlink("target", `\\?\`+filepath.Join(long, "link")); err != nil {
		t.Skipf("cannot make a symlink here: %v", err)
	}
	if got, err := Readlink(filepath.Join(long, "link")); err != nil || got != "target" {
		t.Errorf("Readlink = %q, %v, want %q", got, err, "target")
	}
}

func TestErrorsHoldTheNativePathWithoutThePrefix(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.", "child ")
	for _, wrapper := range missingParentCalls() {
		t.Run(wrapper.name+" names the path as the caller wrote it", func(t *testing.T) {
			text := wrapper.call(missing).Error()
			if !strings.Contains(text, missing) || strings.Contains(text, `\\?\`) {
				t.Errorf("%s error %q, want it to hold %q and no \\\\?\\ prefix", wrapper.name, text, missing)
			}
		})
	}
}
