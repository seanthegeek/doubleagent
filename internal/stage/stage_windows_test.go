package stage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

func TestDestOnWindows(t *testing.T) {
	live := Stager{Dir: "C:/out/.stage"}
	image := Stager{Dir: "C:/out/.stage", Root: "D:/img"}
	cases := []struct {
		name        string
		stager      Stager
		path        string
		wantArchive string
	}{
		{"a live drive path is archived under its drive letter", live,
			"C:/Users/alice/x", "fs/C/Users/alice/x"},
		{"a live drive letter is upper-cased", live, "c:/Users/alice/x", "fs/C/Users/alice/x"},
		{"a live UNC path is archived under its server and share", live,
			"//server/share/x", "fs/server/share/x"},
		{"an image root is removed from the front", image, "D:/img/Users/a", "fs/Users/a"},
		{"an image root is removed ignoring case", Stager{Dir: "C:/s", Root: "d:/IMG"},
			"D:/img/Users/a", "fs/Users/a"},
		{"a path off the image root keeps its drive", image, "E:/other/f", "fs/E/other/f"},
		{"a path under an in-base link spelled in another case is moved",
			image.Through("D:/img/h/.A", "D:/img/h/dot/a"), "D:/img/h/.a/b", "fs/h/dot/a/b"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			archive, dest := test.stager.Dest(test.path)
			if archive != test.wantArchive || dest != test.stager.Dir+"/"+test.wantArchive {
				t.Errorf("Dest(%q) = %q, %q", test.path, archive, dest)
			}
		})
	}
}

// newWindowsStager returns a stager over a fresh staging directory and a
// fresh source root, both canonical, the stager's Physical resolved as the
// run resolves it.
func newWindowsStager(t *testing.T) (Stager, string) {
	t.Helper()
	parent := syspath.FromNative(t.TempDir())
	mustDo(t, os.MkdirAll(parent+"/stage/fs", 0o700))
	mustDo(t, os.MkdirAll(parent+"/root/home", 0o755))
	physical, err := PhysicalDir(parent + "/stage")
	mustDo(t, err)
	return Stager{Dir: parent + "/stage", Physical: physical, Root: parent + "/root"},
		parent + "/root"
}

// makeJunction makes link a junction to the directory target without
// running a program: it creates an empty directory and sets on it the
// mount-point reparse data mklink /J would write. Both paths are canonical
// and absolute.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	mustDo(t, os.Mkdir(link, 0o755))
	name, err := windows.UTF16PtrFromString(syspath.ToNative(link))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	native := syspath.ToNative(target)
	data := mountPointData(`\??\`+native+`\`, native)
	var returned uint32
	mustDo(t, windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil))
}

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

// skipWithoutSymlinkPrivilege skips the test when err says the account may
// not create symlinks.
func skipWithoutSymlinkPrivilege(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skipf("creating a symlink needs SeCreateSymbolicLinkPrivilege or developer mode: %v", err)
	}
}

// setTimes sets the creation, access and write times of path, in that
// order, given as Unix seconds.
func setTimes(t *testing.T, path string, created, accessed, written int64) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	creation := windows.NsecToFiletime(created * 1e9)
	access := windows.NsecToFiletime(accessed * 1e9)
	write := windows.NsecToFiletime(written * 1e9)
	mustDo(t, windows.SetFileTime(handle, &creation, &access, &write))
}

// fileTimes returns the creation, access and write times of path.
func fileTimes(t *testing.T, path string) [3]syscall.Filetime {
	t.Helper()
	info, err := os.Lstat(path)
	mustDo(t, err)
	data := info.Sys().(*syscall.Win32FileAttributeData)
	return [3]syscall.Filetime{data.CreationTime, data.LastAccessTime, data.LastWriteTime}
}

func TestCopyFileOnWindows(t *testing.T) {
	cases := []struct {
		name, directory string
	}{
		{"a short path is copied with its three times", "/home/a"},
		{"a path longer than MAX_PATH is copied with its three times",
			"/home/" + strings.Repeat("long-directory-name/", 15)},
	}
	content := []byte("staged bytes")
	sum := sha256.Sum256(content)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newWindowsStager(t)
			mustDo(t, os.MkdirAll(root+test.directory, 0o755))
			source := strings.TrimSuffix(root+test.directory, "/") + "/file.txt"
			mustDo(t, os.WriteFile(source, content, 0o600))
			setTimes(t, source, 1_500_000_000, 1_600_000_000, 1_700_000_000)
			want := fileTimes(t, source)
			hash, written, err := stager.CopyFile(source)
			mustDo(t, err)
			_, dest := stager.Dest(source)
			staged, err := os.ReadFile(dest)
			mustDo(t, err)
			switch got := fileTimes(t, dest); {
			case hash != hex.EncodeToString(sum[:]) || written != int64(len(content)):
				t.Errorf("hash %s, written %d", hash, written)
			case string(staged) != string(content):
				t.Errorf("staged %q", staged)
			case got != want:
				t.Errorf("times (creation, access, write) %v, want %v", got, want)
			}
		})
	}
}

func TestCopyFileBesideAHandleWithDeleteAccess(t *testing.T) {
	stager, root := newWindowsStager(t)
	source := root + "/home/locked"
	mustDo(t, os.WriteFile(source, []byte("held"), 0o600))
	name, err := windows.UTF16PtrFromString(syspath.ToNative(source))
	mustDo(t, err)
	holder, err := windows.CreateFile(name, windows.GENERIC_READ|windows.DELETE, shareAll, nil,
		windows.OPEN_EXISTING, 0, 0)
	mustDo(t, err)
	defer windows.CloseHandle(holder)
	if file, err := os.Open(source); err == nil {
		_ = file.Close()
		t.Fatal("os.Open shared the file with a delete-access handle; the case tests nothing")
	}
	if _, written, err := stager.CopyFile(source); err != nil || written != 4 {
		t.Errorf("CopyFile = %d bytes, %v; want 4 bytes copied", written, err)
	}
}

func TestCopyFileRefusesAReparsePointDestOnWindows(t *testing.T) {
	cases := []struct {
		name string
		link func(t *testing.T, target, dest string) error
	}{
		{"a junction at the staged path", func(t *testing.T, target, dest string) error {
			makeJunction(t, target, dest)
			return nil
		}},
		{"a symlink at the staged path", func(_ *testing.T, target, dest string) error {
			return createLink(syspath.ToNative(target)+`\keep`, dest, false)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newWindowsStager(t)
			outside := syspath.FromNative(t.TempDir())
			mustDo(t, os.WriteFile(outside+"/keep", []byte("evidence"), 0o600))
			source := root + "/home/f"
			mustDo(t, os.WriteFile(source, []byte("x"), 0o600))
			_, dest := stager.Dest(source)
			mustDo(t, os.MkdirAll(parentOf(dest), 0o700))
			err := test.link(t, outside, dest)
			skipWithoutSymlinkPrivilege(t, err)
			mustDo(t, err)
			if _, _, err := stager.CopyFile(source); !errors.Is(err, notRegular{}) {
				t.Errorf("CopyFile error %v, want not a regular file", err)
			}
			if data, err := os.ReadFile(outside + "/keep"); string(data) != "evidence" {
				t.Errorf("the file beyond the link changed: %q, %v", data, err)
			}
		})
	}
}

func TestNoWriteThroughARecreatedJunction(t *testing.T) {
	stager, root := newWindowsStager(t)
	home := root + "/home/dot"
	mustDo(t, os.MkdirAll(home+"/dotfiles/claude/proj", 0o755))
	source := home + "/dotfiles/claude/proj/AGENTS.md"
	mustDo(t, os.WriteFile(source, []byte("x\n"), 0o600))
	mustDo(t, stager.MakeParent(home+"/.claude"))
	_, staged := stager.Dest(home + "/.claude")
	makeJunction(t, home+"/dotfiles/claude", staged)
	through := home + "/.claude/proj/AGENTS.md"
	cases := []struct {
		name  string
		write func() error
	}{
		{"a copy into the junction is refused", func() error {
			_, _, err := stager.CopyFile(through)
			return err
		}},
		{"a directory under the junction is not made", func() error {
			return stager.MakeParent(home + "/.claude/proj/new/f")
		}},
		{"a link under the junction is not created", func() error {
			return stager.Symlink(home+"/.claude/proj/link", "target", false)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := test.write()
			var pathError *fs.PathError
			if !errors.As(err, &pathError) || !errors.Is(pathError.Err, outsideStage{}) {
				t.Errorf("error %v, want one saying the staging directory is left", err)
			}
			entries, _ := os.ReadDir(home + "/dotfiles/claude/proj")
			if data, _ := os.ReadFile(source); string(data) != "x\n" || len(entries) != 1 {
				t.Errorf("source changed: %q, %d entries", data, len(entries))
			}
		})
	}
}

func TestPhysicalDirOnWindows(t *testing.T) {
	parent, err := PhysicalDir(syspath.FromNative(t.TempDir()))
	mustDo(t, err)
	mustDo(t, os.MkdirAll(parent+"/real/sub", 0o755))
	mustDo(t, os.WriteFile(parent+"/file", nil, 0o600))
	makeJunction(t, parent+"/real", parent+"/junction")
	cases := []struct {
		name, dir, want string
	}{
		{"a junction is resolved", parent + "/junction/sub", parent + "/real/sub"},
		{"a native path through a junction is resolved",
			syspath.ToNative(parent + "/junction/sub"), parent + "/real/sub"},
		{"a directory spelled in another case gets its on-disk case",
			parent + "/REAL/SUB", parent + "/real/sub"},
		{"a file is not a directory", parent + "/file", ""},
		{"a missing directory fails", parent + "/missing", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := PhysicalDir(test.dir)
			if got != test.want || (err == nil) != (test.want != "") {
				t.Errorf("PhysicalDir(%q) = %q, %v; want %q", test.dir, got, err, test.want)
			}
		})
	}
	t.Run("a file gives the not-a-directory error naming the native path", func(t *testing.T) {
		_, err := PhysicalDir(parent + "/file")
		var pathError *fs.PathError
		if !errors.As(err, &pathError) || pathError.Err != syscall.ENOTDIR ||
			pathError.Path != syspath.ToNative(parent+"/file") {
			t.Errorf("error %v", err)
		}
	})
}

func TestSymlinkKindOnWindows(t *testing.T) {
	cases := []struct {
		name      string
		directory bool
	}{
		{"a directory link is made for a directory source", true},
		{"a file link is made for a file source", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newWindowsStager(t)
			path := root + "/home/link"
			mustDo(t, stager.MakeParent(path))
			err := stager.Symlink(path, `..\missing target`, test.directory)
			skipWithoutSymlinkPrivilege(t, err)
			mustDo(t, err)
			_, dest := stager.Dest(path)
			meta, err := fsmeta.Lstat(dest)
			mustDo(t, err)
			target, err := os.Readlink(dest)
			mustDo(t, err)
			if meta.Kind != fsmeta.Symlink || meta.DirectoryLink() != test.directory ||
				target != `..\missing target` {
				t.Errorf("kind %v, directory link %v, target %q", meta.Kind,
					meta.DirectoryLink(), target)
			}
		})
	}
}

func TestRemoveFailedCopyOnWindows(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(t *testing.T, stager Stager, outside string) string
		wantResult  bool
		wantPresent bool
	}{
		{"a partial copy is removed when the staging path differs in case",
			func(t *testing.T, stager Stager, _ string) string {
				mustDo(t, os.WriteFile(stager.Dir+"/fs/partial", []byte("x"), 0o600))
				return stager.Dir + "/fs/partial"
			}, true, false},
		{"a file reached through a junction out of the staging tree is left",
			func(t *testing.T, stager Stager, outside string) string {
				mustDo(t, os.WriteFile(outside+"/source", []byte("x"), 0o600))
				makeJunction(t, outside, stager.Dir+"/fs/junction")
				return stager.Dir + "/fs/junction/source"
			}, false, true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, _ := newWindowsStager(t)
			stager.Physical = syspath.Fold(stager.Physical)
			dest := test.setup(t, stager, syspath.FromNative(t.TempDir()))
			if got := stager.RemoveFailedCopy(dest); got != test.wantResult {
				t.Errorf("RemoveFailedCopy = %v", got)
			}
			if _, err := os.Lstat(dest); (err == nil) != test.wantPresent {
				t.Errorf("present after: %v", err == nil)
			}
		})
	}
}

func TestOutsideStageErrorNamesTheNativePath(t *testing.T) {
	stager, _ := newWindowsStager(t)
	outside := syspath.FromNative(t.TempDir()) + "/outside/deeper"
	err := stager.requireInside("mkdir", outside)
	var pathError *fs.PathError
	if !errors.As(err, &pathError) || pathError.Path != syspath.ToNative(outside) {
		t.Errorf("requireInside error %v, want a PathError of %q", err, syspath.ToNative(outside))
	}
}
