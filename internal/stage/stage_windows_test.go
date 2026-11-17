package stage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/shortname"
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
			archive, dest, _ := test.stager.Dest(test.path)
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
	return Stager{Dir: parent + "/stage", Physical: physical, Root: parent + "/root",
		Places: NewPlaces()}, parent + "/root"
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

func TestCopyOnWindows(t *testing.T) {
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
			hash, written, err := copyFile(stager, source)
			mustDo(t, err)
			_, dest, _ := stager.Dest(source)
			// The times are read before the bytes: where the volume
			// records access times, the test's own read moves the copy's.
			got := fileTimes(t, dest)
			staged, err := os.ReadFile(dest)
			mustDo(t, err)
			switch {
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

// basicTimes returns the creation, access, write and change times of path,
// the change time being the last change to the file's record, which no
// other call shows; opening for attributes alone records no access.
func basicTimes(t *testing.T, path string) [4]int64 {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	// FILE_BASIC_INFO: four times, the attributes, and padding to eight bytes.
	var basic struct {
		times      [4]int64
		attributes uint32
		_          uint32
	}
	mustDo(t, windows.GetFileInformationByHandleEx(handle, windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic))))
	return basic.times
}

// refuseTimeSetting gives path an access list that lets the current user
// read it and nothing else, so an open asking to set its times is refused,
// and restores full access when the test ends. The test is skipped when the
// list cannot be set or does not refuse that open.
func refuseTimeSetting(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Skipf("cannot read the process token: %v", err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.FILE_GENERIC_READ,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
			TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)},
	}}, nil)
	if err != nil {
		t.Skipf("cannot build an access control list: %v", err)
	}
	native := syspath.ToNative(path)
	information := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION |
		windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if err := windows.SetNamedSecurityInfo(native, windows.SE_FILE_OBJECT, information,
		nil, nil, acl, nil); err != nil {
		t.Skipf("cannot set an access control list here: %v", err)
	}
	t.Cleanup(func() {
		_ = windows.SetNamedSecurityInfo(native, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION, nil, nil, nil, nil)
	})
	name, err := windows.UTF16PtrFromString(native)
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err == nil {
		_ = windows.CloseHandle(handle)
		t.Skip("the access list did not refuse setting the times (a privilege overrides it?)")
	}
}

// fileUSN returns the number of the last change-journal record written for
// path, read with FSCTL_READ_FILE_USN_DATA through a handle opened for
// attributes alone, or the error that says the volume keeps no journal.
// golang.org/x/sys defines no USN record type: the record is USN_RECORD_V2
// (Usn at byte 24) or USN_RECORD_V3 (file numbers of 16 bytes, Usn at byte
// 40), told apart by MajorVersion at byte 4.
func fileUSN(t *testing.T, path string) (int64, error) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	// READ_FILE_USN_DATA: the lowest and highest record versions accepted.
	versions := [2]uint16{2, 3}
	record := make([]byte, 1024)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_READ_FILE_USN_DATA,
		(*byte)(unsafe.Pointer(&versions[0])), uint32(unsafe.Sizeof(versions)),
		&record[0], uint32(len(record)), &returned, nil)
	if err != nil {
		return 0, err
	}
	switch major := binary.LittleEndian.Uint16(record[4:]); {
	case major == 2 && returned >= 32:
		return int64(binary.LittleEndian.Uint64(record[24:])), nil
	case major == 3 && returned >= 48:
		return int64(binary.LittleEndian.Uint64(record[40:])), nil
	default:
		t.Fatalf("change-journal record of version %d, %d bytes", major, returned)
		return 0, nil
	}
}

func TestCopyLeavesTheSourceAsItWas(t *testing.T) {
	cases := []struct {
		name string
		// prepare changes the source before the copy; holds says the
		// source keeps its access time where the volume records reads.
		prepare func(t *testing.T, source string)
		holds   bool
	}{
		{"a file keeps its four times", func(*testing.T, string) {}, true},
		{"a read-only file keeps its four times", func(t *testing.T, source string) {
			mustDo(t, os.Chmod(source, 0o444))
			t.Cleanup(func() { _ = os.Chmod(source, 0o600) })
		}, true},
		{"a file whose times may not be set is still copied", refuseTimeSetting, false},
	}
	content := []byte("evidence")
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newWindowsStager(t)
			source := root + "/home/file.txt"
			mustDo(t, os.WriteFile(source, content, 0o600))
			// An access time far in the past, which NTFS would move on a
			// read even with its one-hour update granularity.
			setTimes(t, source, 1_500_000_000, 1_600_000_000, 1_700_000_000)
			test.prepare(t, source)
			before := basicTimes(t, source)
			usnBefore, journalErr := fileUSN(t, source)
			if journalErr == nil && usnBefore == 0 {
				journalErr = errors.New("the file has no journal record (USN 0)")
			}
			_, written, err := copyFile(stager, source)
			if err != nil || written != int64(len(content)) {
				t.Fatalf("Copy = %d bytes, %v", written, err)
			}
			after := basicTimes(t, source)
			if !test.holds {
				after[1] = before[1]
			}
			if after != before {
				t.Errorf("source times (creation, access, write, change) %v, want %v",
					after, before)
			}
			// The hold is a SetFileTime call; it must leave no change-journal
			// record (USN_REASON_BASIC_INFO_CHANGE) on the evidence.
			if journalErr != nil {
				t.Skipf("no change journal to read here: %v", journalErr)
			}
			usnAfter, err := fileUSN(t, source)
			t.Logf("source USN before the copy %d, after %d", usnBefore, usnAfter)
			if err != nil || usnAfter != usnBefore {
				t.Errorf("source USN %d (%v) after the copy, want %d", usnAfter, err, usnBefore)
			}
		})
	}
}

func TestCopyWithReadOnlySources(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, source string)
	}{
		{"a file is copied with its three times", func(*testing.T, string) {}},
		{"a file whose times may not be set is copied with its three times", refuseTimeSetting},
	}
	content := []byte("evidence")
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			stager, root := newWindowsStager(t)
			stager.Opener = fsmeta.NewOpener(true, false)
			source := root + "/home/file.txt"
			mustDo(t, os.WriteFile(source, content, 0o600))
			setTimes(t, source, 1_500_000_000, 1_600_000_000, 1_700_000_000)
			want := fileTimes(t, source)
			before := basicTimes(t, source)
			test.prepare(t, source)
			_, written, err := copyFile(stager, source)
			if err != nil || written != int64(len(content)) {
				t.Fatalf("Copy = %d bytes, %v", written, err)
			}
			_, dest, _ := stager.Dest(source)
			if got := fileTimes(t, dest); got != want {
				t.Errorf("copy times (creation, access, write) %v, want %v", got, want)
			}
			// Reading may move the access time, and the access list the
			// change time; the creation and write times stay.
			after := basicTimes(t, source)
			if after[0] != before[0] || after[2] != before[2] {
				t.Errorf("source times (creation, access, write, change) %v, want the creation and "+
					"write times of %v", after, before)
			}
		})
	}
}

// skipWithoutAccessUpdates skips the test unless a plain read of a scratch
// file in dir moves its last-access time: where Windows has last-access
// updates off for the volume, no read records one.
func skipWithoutAccessUpdates(t *testing.T, dir string) {
	t.Helper()
	probe := dir + "/access-probe"
	mustDo(t, os.WriteFile(probe, []byte("probe"), 0o600))
	setTimes(t, probe, 1_500_000_000, 1_600_000_000, 1_700_000_000)
	before := basicTimes(t, probe)
	_, err := os.ReadFile(probe)
	mustDo(t, err)
	if basicTimes(t, probe)[1] == before[1] {
		t.Skip("last-access updates are off on this volume: a read did not move the access time")
	}
}

func TestReadOnlySourcesLetTheReadMoveTheAccessTime(t *testing.T) {
	stager, root := newWindowsStager(t)
	skipWithoutAccessUpdates(t, root)
	stager.Opener = fsmeta.NewOpener(true, false)
	source := root + "/home/file.txt"
	mustDo(t, os.WriteFile(source, []byte("evidence"), 0o600))
	setTimes(t, source, 1_500_000_000, 1_600_000_000, 1_700_000_000)
	want := fileTimes(t, source)
	before := basicTimes(t, source)
	if _, _, err := copyFile(stager, source); err != nil {
		t.Fatal(err)
	}
	after := basicTimes(t, source)
	_, dest, _ := stager.Dest(source)
	if got := fileTimes(t, dest); got != want {
		t.Errorf("copy times (creation, access, write) %v, want the source's %v", got, want)
	}
	if after[1] == before[1] {
		t.Errorf("source access time %d did not move; a read-only open cannot hold it", after[1])
	}
	if after[0] != before[0] || after[2] != before[2] {
		t.Errorf("source times (creation, access, write) %v, want creation and write of %v",
			after, before)
	}
}

func TestCopyBesideAHandleWithDeleteAccess(t *testing.T) {
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
	if _, written, err := copyFile(stager, source); err != nil || written != 4 {
		t.Errorf("Copy = %d bytes, %v; want 4 bytes copied", written, err)
	}
}

func TestCopyRefusesAReparsePointDestOnWindows(t *testing.T) {
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
			_, dest, _ := stager.Dest(source)
			mustDo(t, os.MkdirAll(parentOf(dest), 0o700))
			err := test.link(t, outside, dest)
			skipWithoutSymlinkPrivilege(t, err)
			mustDo(t, err)
			if _, _, err := copyFile(stager, source); !errors.Is(err, notRegular{}) {
				t.Errorf("Copy error %v, want not a regular file", err)
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
	_, staged, _ := stager.Dest(home + "/.claude")
	makeJunction(t, home+"/dotfiles/claude", staged)
	through := home + "/.claude/proj/AGENTS.md"
	cases := []struct {
		name  string
		write func() error
	}{
		{"a copy into the junction is refused", func() error {
			_, _, err := copyFile(stager, through)
			return err
		}},
		{"a directory under the junction is not made", func() error {
			return stager.MakeParent(home + "/.claude/proj/new/f")
		}},
		{"a link under the junction is not created", func() error {
			return symlink(stager, home+"/.claude/proj/link", "target", false)
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
			err := symlink(stager, path, `..\missing target`, test.directory)
			skipWithoutSymlinkPrivilege(t, err)
			mustDo(t, err)
			_, dest, _ := stager.Dest(path)
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

func TestFailedCopyInsideOnWindows(t *testing.T) {
	cases := []struct {
		name        string
		setup       func(t *testing.T, stager Stager, outside string) string
		wantResult  bool
		wantPresent bool
	}{
		{"a file in the staging directory is inside when its physical path differs in case",
			func(t *testing.T, stager Stager, _ string) string {
				mustDo(t, os.WriteFile(stager.Dir+"/fs/partial", []byte("x"), 0o600))
				return stager.Dir + "/fs/partial"
			}, true, true},
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
			if got := stager.FailedCopyInside(dest); got != test.wantResult {
				t.Errorf("FailedCopyInside = %v", got)
			}
			if _, err := os.Lstat(dest); (err == nil) != test.wantPresent {
				t.Errorf("present after: %v", err == nil)
			}
		})
	}
}

func TestAFailedCopyRemovesItsOwnFileWhenTheStagingPathDiffersInCase(t *testing.T) {
	stager, root := newWindowsStager(t)
	stager.Physical = syspath.Fold(stager.Physical)
	source := root + "/home/f"
	mustDo(t, os.WriteFile(source, []byte("x"), 0o600))
	stager.wrapWriter = func(io.Writer) io.Writer { return failingWriter{} }
	if _, err := stager.Copy(source); err == nil {
		t.Fatal("the copy did not fail")
	}
	_, dest, _ := stager.Dest(source)
	if _, err := os.Lstat(dest); err == nil {
		t.Error("the failed copy's partial file is left in the staging directory")
	}
}

// failingWriter fails every write.
type failingWriter struct{}

// Write fails.
func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestOutsideStageErrorNamesTheNativePath(t *testing.T) {
	stager, _ := newWindowsStager(t)
	outside := syspath.FromNative(t.TempDir()) + "/outside/deeper"
	err := stager.requireInside("mkdir", outside)
	var pathError *fs.PathError
	if !errors.As(err, &pathError) || pathError.Path != syspath.ToNative(outside) {
		t.Errorf("requireInside error %v, want a PathError of %q", err, syspath.ToNative(outside))
	}
}

func TestHoldsComparesPhysicalPathsByteForByte(t *testing.T) {
	base := syspath.FromNative(t.TempDir())
	for _, name := range []string{".stage-x/fs", ".ſtage-x/fs"} {
		mustDo(t, os.MkdirAll(syspath.Extended(base+"/"+name), 0o755))
	}
	stager := Stager{Dir: base + "/.stage-x", Physical: base + "/.stage-x"}
	cases := []struct {
		name, physical string
		want           bool
	}{
		{"the staging directory as the disk spells it is held", base + "/.stage-x/fs", true},
		// Physical paths are disk spellings, so another spelling names
		// another directory, as it would in a case-sensitive directory.
		{"a physical path differing only in ASCII case is not held", base + "/.STAGE-X/fs", false},
		{"a long-s sibling NTFS keeps apart is not held", base + "/.ſtage-x/fs", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := stager.holds(test.physical); got != test.want {
				t.Errorf("holds(%q) = %t, want %t", test.physical, got, test.want)
			}
		})
	}
}

func TestCopyRefusesAPathWithDotDot(t *testing.T) {
	stager, root := newWindowsStager(t)
	mustDo(t, os.WriteFile(syspath.ToNative(root+"/home/f"), []byte("source"), 0o644))
	for _, path := range []string{root + "/home/../home/f", root + "/home/./f",
		root + "/../root/home/f"} {
		if _, _, err := copyFile(stager, path); err == nil {
			t.Errorf("Copy(%s) copied it", path)
		}
		if err := stager.MakeParent(path); err == nil {
			t.Errorf("MakeParent(%s) made its parent", path)
		}
	}
	entries, err := os.ReadDir(syspath.ToNative(stager.Dir + "/fs"))
	if err != nil || len(entries) != 0 {
		t.Errorf("the staging tree holds %v (%v), want nothing", entries, err)
	}
	if text, err := os.ReadFile(syspath.ToNative(root + "/home/f")); err != nil ||
		string(text) != "source" {
		t.Errorf("the source reads %q, %v", text, err)
	}
}

func TestADirectoryNamedAsAnotherDirectorysShortNameIsMergedSafely(t *testing.T) {
	// Measured on the CI runner: NTFS makes 8.3 short names, and opens a
	// path through one, in a directory marked case-sensitive as in any
	// other, so both variants merge the two directories.
	for _, marked := range []bool{false, true} {
		t.Run(fmt.Sprintf("staging directory marked case-sensitive %v", marked), func(t *testing.T) {
			stagesUnderAShortName(t, marked)
		})
	}
}

// stagesUnderAShortName stages .claude-backup/x from one root, then from a
// second root x and y under a directory literally named as the staged
// directory's 8.3 alias, into a staging fs directory marked case-sensitive
// or not, and checks the collision and the members.
func stagesUnderAShortName(t *testing.T, marked bool) {
	first, root := newWindowsStager(t)
	if marked {
		mustDo(t, MarkCaseSensitive(first.Dir+"/fs"))
	}
	second := first
	second.Root = root + "-two"
	long := root + "/.claude-backup"
	mustDo(t, os.MkdirAll(long, 0o755))
	mustDo(t, os.WriteFile(long+"/x", []byte("first entry"), 0o600))
	one, err := first.Copy(long + "/x")
	mustDo(t, err)
	stagedDir := first.Dir + "/" + one.ArchivePath[:strings.LastIndexByte(one.ArchivePath, '/')]
	short := syspath.FromNative(shortname.ShortPath(t, syspath.ToNative(stagedDir)))
	alias := short[strings.LastIndexByte(short, '/')+1:]
	if strings.EqualFold(alias, ".claude-backup") {
		t.Skipf("the staging volume gives %s no 8.3 short name", stagedDir)
	}
	other := second.Root + "/" + alias
	mustDo(t, os.MkdirAll(other, 0o755))
	mustDo(t, os.WriteFile(other+"/x", []byte("second entry"), 0o600))
	mustDo(t, os.WriteFile(other+"/y", []byte("second entry's y"), 0o600))
	if _, err := second.Copy(other + "/x"); !IsCollision(err) ||
		!strings.Contains(err.Error(), "takes its name for another file's, link's or directory's; ") {
		t.Errorf("%s/x = %v; want the collision reason", alias, err)
	}
	why, err := second.Copy(other + "/y")
	mustDo(t, err)
	for archived, text := range map[string]string{one.ArchivePath: "first entry",
		why.ArchivePath: "second entry's y"} {
		if data, err := os.ReadFile(syspath.ToNative(first.Dir + "/" + archived)); err != nil ||
			string(data) != text {
			t.Errorf("member %s holds %q (%v), want %q", archived, data, err, text)
		}
		if spelled := fsmeta.OnDiskPath(first.Dir, first.Dir+"/"+archived); spelled !=
			first.Dir+"/"+archived {
			t.Errorf("archive path %s names a member the staging tree spells %s", archived, spelled)
		}
	}
}
