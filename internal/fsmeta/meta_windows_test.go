package fsmeta

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// mountPointData lays out a REPARSE_DATA_BUFFER holding a mount-point
// (junction) reparse buffer: the tag, the data length, a reserved word, the
// offsets and lengths in bytes of the two names, then the substitute name
// the system follows and the print name shown to people, each ending in NUL.
func mountPointData(substitute, printName string) []byte {
	substituteText := utf16.Encode([]rune(substitute))
	printText := utf16.Encode([]rune(printName))
	names := make([]uint16, 0, len(substituteText)+len(printText)+2)
	names = append(append(names, substituteText...), 0)
	names = append(append(names, printText...), 0)
	substituteBytes := uint16(2 * len(substituteText))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_MOUNT_POINT)
	data = binary.LittleEndian.AppendUint16(data, uint16(8+2*len(names)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes+2)
	data = binary.LittleEndian.AppendUint16(data, uint16(2*len(printText)))
	for _, unit := range names {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

// makeJunction makes link a junction to the directory target without
// running a program: it creates an empty directory and sets on it the
// mount-point reparse data mklink /J would write. Both paths are absolute.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(link, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, noFollowFlags, 0)
	if err != nil {
		t.Fatalf("open %s: %v", link, err)
	}
	defer windows.CloseHandle(handle)
	data := mountPointData(`\??\`+target+`\`, target)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if err != nil {
		t.Fatalf("set the junction data on %s: %v", link, err)
	}
}

// symlinkOrSkip makes link a symlink to target, skipping the test when the
// account lacks the privilege to create symlinks.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skipf("creating a symlink needs SeCreateSymbolicLinkPrivilege or developer mode: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// writeFile writes text to path, failing the test on an error.
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLstatKindsOnWindows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "file"), "hello")
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, filepath.Join(root, "dir"), filepath.Join(root, "junction"))
	cases := []struct {
		name, path     string
		kind           Kind
		directoryLink  bool
		attributeNames string
	}{
		{"a file is a file", "file", File, false, "Archive"},
		{"a directory is a directory", "dir", Dir, false, "Directory"},
		{"a junction is a directory link", "junction", Symlink, true, "Directory, ReparsePoint"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			meta, err := Lstat(syspath.FromNative(filepath.Join(root, testCase.path)))
			if err != nil {
				t.Fatal(err)
			}
			if meta.Kind != testCase.kind || meta.DirectoryLink() != testCase.directoryLink {
				t.Errorf("kind %v, directory link %v; want %v, %v", meta.Kind,
					meta.DirectoryLink(), testCase.kind, testCase.directoryLink)
			}
			got := AttributesString(meta.Attributes &^ windows.FILE_ATTRIBUTE_NOT_CONTENT_INDEXED)
			if got != testCase.attributeNames {
				t.Errorf("attributes %q, want %q", got, testCase.attributeNames)
			}
			if meta.Mode != "" || meta.Ctime != 0 || meta.UID != 0 || meta.GID != 0 {
				t.Errorf("mode %q ctime %d uid %d gid %d; want empty and zeros", meta.Mode,
					meta.Ctime, meta.UID, meta.GID)
			}
		})
	}
}

func TestLstatSymlinksOnWindows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "file"), "hello")
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, filepath.Join(root, "file"), filepath.Join(root, "filelink"))
	symlinkOrSkip(t, filepath.Join(root, "dir"), filepath.Join(root, "dirlink"))
	cases := []struct {
		name, path    string
		directoryLink bool
	}{
		{"a file symlink is a link but not a directory link", "filelink", false},
		{"a directory symlink is a directory link", "dirlink", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			meta, err := Lstat(syspath.FromNative(filepath.Join(root, testCase.path)))
			if err != nil {
				t.Fatal(err)
			}
			if meta.Kind != Symlink || meta.DirectoryLink() != testCase.directoryLink {
				t.Errorf("kind %v, directory link %v; want Symlink, %v", meta.Kind,
					meta.DirectoryLink(), testCase.directoryLink)
			}
		})
	}
}

// listenUnix makes an AF_UNIX socket at each of names, relative to dir, from
// inside dir so that the name Go hands the system stays under its 108-byte
// limit. It skips the test only where the system does not support AF_UNIX
// sockets, and fails it on any other error.
func listenUnix(t *testing.T, dir string, names ...string) {
	t.Helper()
	t.Chdir(syspath.ToNative(dir))
	for _, name := range names {
		listener, err := net.Listen("unix", syspath.ToNative(name))
		if errors.Is(err, windows.WSAEAFNOSUPPORT) || errors.Is(err, windows.WSAEPROTONOSUPPORT) {
			t.Skipf("this system does not support AF_UNIX sockets: %v", err)
		}
		if err != nil {
			t.Fatalf("creating an AF_UNIX socket at %s in %s: %v", name, dir, err)
		}
		t.Cleanup(func() { _ = listener.Close() })
	}
}

func TestLstatSocketIsOther(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "socket")
	listenUnix(t, syspath.FromNative(dir), "socket")
	meta, err := Lstat(syspath.FromNative(path))
	if err != nil || meta.Kind != Other {
		t.Errorf("Lstat(socket) = kind %v, error %v; want Other", meta.Kind, err)
	}
}

func TestLstatIdentityOnWindows(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "First")
	second := filepath.Join(root, "second")
	writeFile(t, first, "one")
	writeFile(t, second, "two")
	lstat := func(path string) Meta {
		t.Helper()
		meta, err := Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		return meta
	}
	original := lstat(syspath.FromNative(first))
	respelled := lstat(strings.ToLower(syspath.FromNative(first)))
	native := lstat(first)
	other := lstat(syspath.FromNative(second))
	t.Run("two spellings of one file have one identity", func(t *testing.T) {
		if original.Ino == 0 || original.Dev != respelled.Dev || original.Ino != respelled.Ino ||
			original.Ino != native.Ino {
			t.Errorf("identities %d/%d, %d/%d, %d/%d; want one nonzero", original.Dev,
				original.Ino, respelled.Dev, respelled.Ino, native.Dev, native.Ino)
		}
	})
	t.Run("two files have two identities", func(t *testing.T) {
		if original.Ino == other.Ino {
			t.Errorf("both files have index %d", original.Ino)
		}
	})
	t.Run("a file has one link", func(t *testing.T) {
		if original.Nlink != 1 || original.Size != 3 {
			t.Errorf("links %d, size %d; want 1, 3", original.Nlink, original.Size)
		}
	})
}

func TestLstatTimesOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	writeFile(t, path, "text")
	modified := time.Unix(1_700_000_000, 0)
	accessed := time.Unix(1_600_000_000, 0)
	if err := os.Chtimes(path, accessed, modified); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes := info.Sys().(*syscall.Win32FileAttributeData)
	created := attributes.CreationTime.Nanoseconds() / 1e9
	meta, err := Lstat(syspath.FromNative(path))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Mtime != modified.Unix() || meta.Atime != accessed.Unix() || meta.Btime != created {
		t.Errorf("times %d %d %d; want %d %d %d", meta.Mtime, meta.Atime, meta.Btime,
			modified.Unix(), accessed.Unix(), created)
	}
}

// listedEntry returns what the listing of path's parent directory says of
// path: its write time, access time, creation time and size, the directory
// entry a listing reads.
func listedEntry(t *testing.T, path string) [4]int64 {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(path) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		data := info.Sys().(*syscall.Win32FileAttributeData)
		return [4]int64{epochSeconds(windows.Filetime(data.LastWriteTime)),
			epochSeconds(windows.Filetime(data.LastAccessTime)),
			epochSeconds(windows.Filetime(data.CreationTime)), info.Size()}
	}
	t.Fatalf("the listing of %s has no %s", filepath.Dir(path), filepath.Base(path))
	return [4]int64{}
}

// makeStaleDirectory makes a directory dated an hour ago, describes it, then
// creates a child in it, which updates its record before its entry.
func makeStaleDirectory(t *testing.T, directory string) {
	t.Helper()
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(directory, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Lstat(syspath.FromNative(directory)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(directory, "child"), "text")
}

// makeStaleLink makes second a hard link to first, then appends to the file
// through first, which leaves the entry of second as it was.
func makeStaleLink(t *testing.T, first, second string) {
	t.Helper()
	writeFile(t, first, "one")
	if err := os.Link(first, second); err != nil {
		t.Fatal(err)
	}
	handle, err := os.OpenFile(first, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handle.WriteString("more text")
	if closeError := handle.Close(); err == nil {
		err = closeError
	}
	if err != nil {
		t.Fatal(err)
	}
}

// The directory entry can lag the file's record: a directory's entry in its
// parent after a child is created, the entry of one hard link after the file
// is written through another. Lstat reads the record.
func TestLstatReadsTheFileItself(t *testing.T) {
	root := t.TempDir()
	directory, first, second := filepath.Join(root, "directory"), filepath.Join(root, "first"),
		filepath.Join(root, "second")
	makeStaleDirectory(t, directory)
	makeStaleLink(t, first, second)
	t.Run("a hard link written through another name has the file's size", func(t *testing.T) {
		meta, err := Lstat(syspath.FromNative(second))
		if err != nil || meta.Size != int64(len("onemore text")) {
			t.Errorf("size %d, error %v; want %d", meta.Size, err, len("onemore text"))
		}
	})
	t.Run("a directory that gained a child has the child's write time", func(t *testing.T) {
		meta, err := Lstat(syspath.FromNative(directory))
		if err != nil || meta.Mtime < time.Now().Add(-time.Minute).Unix() {
			t.Errorf("write time %d, error %v; want the last minute", meta.Mtime, err)
		}
	})
	t.Run("the identity of a hard link is the file's", func(t *testing.T) {
		one, oneError := Lstat(syspath.FromNative(first))
		two, twoError := Lstat(syspath.FromNative(second))
		if oneError != nil || twoError != nil || one.Ino == 0 || one.Ino != two.Ino ||
			two.Nlink != 2 {
			t.Errorf("indexes %d, %d, links %d, errors %v, %v; want one nonzero index, 2 links",
				one.Ino, two.Ino, two.Nlink, oneError, twoError)
		}
	})
}

// The page file cannot be opened even with no access right, so Lstat falls
// back to its directory entry and leaves its identity 0.
func TestLstatDescribesAFileNoHandleOpensFromItsEntry(t *testing.T) {
	path := filepath.Join(os.Getenv("SystemDrive")+`\`, "pagefile.sys")
	name, err := windows.UTF16PtrFromString(syspath.Extended(syspath.FromNative(path)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lstatHandle(name); err == nil || os.IsNotExist(err) {
		t.Skipf("%s is missing or opens (%v); the test needs a file no handle opens", path, err)
	}
	meta, err := Lstat(syspath.FromNative(path))
	if err != nil {
		t.Fatal(err)
	}
	listed := listedEntry(t, path)
	if meta.Kind != File || meta.Size != listed[3] || meta.Mtime != listed[0] || meta.Ino != 0 {
		t.Errorf("kind %v, size %d, write time %d, index %d; want a file of %d bytes written "+
			"at %d with index 0", meta.Kind, meta.Size, meta.Mtime, meta.Ino, listed[3], listed[0])
	}
}

func TestLstatErrorsOnWindows(t *testing.T) {
	missing := syspath.FromNative(filepath.Join(t.TempDir(), "missing"))
	_, err := Lstat(missing)
	var pathError *os.PathError
	if !os.IsNotExist(err) || !errors.As(err, &pathError) || pathError.Op != "lstat" ||
		pathError.Path != syspath.ToNative(missing) {
		t.Errorf("Lstat(missing) error %v; want a not-exist lstat error naming %s", err,
			syspath.ToNative(missing))
	}
}

func TestLstatDriveRoot(t *testing.T) {
	drive := filepath.VolumeName(t.TempDir())
	meta, err := Lstat(drive)
	if err != nil || meta.Kind != Dir {
		t.Errorf("Lstat(%s) = kind %v, error %v; want a directory", drive, meta.Kind, err)
	}
}

func TestHiddenAttribute(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hidden")
	writeFile(t, path, "")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(name, windows.FILE_ATTRIBUTE_HIDDEN); err != nil {
		t.Fatal(err)
	}
	meta, err := Lstat(syspath.FromNative(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := AttributesString(meta.Attributes); got != "Hidden" {
		t.Errorf("attributes %q, want Hidden", got)
	}
}

// tokenOwner returns the account name of the owner this process gives the
// files it creates, the token's default owner: the user, or the
// Administrators group for an elevated administrator.
func tokenOwner(t *testing.T) string {
	t.Helper()
	token := windows.GetCurrentProcessToken()
	var size uint32
	_ = windows.GetTokenInformation(token, windows.TokenOwner, nil, 0, &size)
	information := make([]byte, size)
	err := windows.GetTokenInformation(token, windows.TokenOwner, &information[0], size, &size)
	if err != nil {
		t.Fatal(err)
	}
	owner := (*struct{ Owner *windows.SID })(unsafe.Pointer(&information[0])).Owner
	account, domain, _, err := owner.LookupAccount("")
	if err != nil {
		t.Fatal(err)
	}
	if domain == "" {
		return account
	}
	return domain + `\` + account
}

func TestOwnersName(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	writeFile(t, file, "text")
	makeJunction(t, root, filepath.Join(root, "junction"))
	want := tokenOwner(t)
	owners := NewOwners()
	cases := []struct {
		name, path, want string
	}{
		{"a file is owned by the token owner", syspath.FromNative(file), want},
		{"the cached name is the same", syspath.FromNative(file), want},
		{"a junction has its own owner", syspath.FromNative(filepath.Join(root, "junction")), want},
		{"a missing file has no owner", syspath.FromNative(filepath.Join(root, "missing")), ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := owners.Name(testCase.path); got != testCase.want {
				t.Errorf("Name(%s) = %q, want %q", testCase.path, got, testCase.want)
			}
		})
	}
}

func TestDirReadErrorOnWindows(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	writeFile(t, root+"/file", "")
	cases := []struct {
		name, path, want string
		failed           bool
	}{
		{"a readable directory has no error", root, "", false},
		{"a file is not a directory", root + "/file",
			"open " + syspath.ToNative(root+"/file") + ": not a directory", true},
		{"a missing path gives the lstat error", root + "/missing",
			"lstat " + syspath.ToNative(root+"/missing") + ": ", true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			reason, failed, _ := DirReadError(nil, testCase.path)
			if failed != testCase.failed || !strings.HasPrefix(reason, testCase.want) {
				t.Errorf("DirReadError = %q, %v; want %q..., %v", reason, failed,
					testCase.want, testCase.failed)
			}
		})
	}
}

func TestOpenRegularRefusesAJunction(t *testing.T) {
	root := t.TempDir()
	makeJunction(t, root, filepath.Join(root, "junction"))
	writeFile(t, filepath.Join(root, "file"), "text")
	if _, err := (&Opener{}).OpenRegular(filepath.Join(root, "junction")); !errors.Is(err, ErrNotRegular) {
		t.Errorf("OpenRegular(junction) error %v, want ErrNotRegular", err)
	}
	handle, err := (&Opener{}).OpenRegular(filepath.Join(root, "file"))
	if err != nil {
		t.Fatal(err)
	}
	_ = handle.Close()
}
