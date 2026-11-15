package fsmeta

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

func TestOpenRegularErrorNamesTheNativePath(t *testing.T) {
	canonical := syspath.FromNative(t.TempDir())
	_, err := (&Opener{}).OpenRegular(canonical)
	if !errors.Is(err, ErrNotRegular) {
		t.Fatalf("OpenRegular(directory) error %v, want ErrNotRegular", err)
	}
	text := err.Error()
	if !strings.Contains(text, syspath.ToNative(canonical)) || strings.Contains(text, "/") ||
		strings.Contains(text, `\\?\`) {
		t.Errorf("error %q, want the backslashed path %q and no slash or \\\\?\\ prefix",
			text, syspath.ToNative(canonical))
	}
}

// tempFile creates a file in a new temporary directory and returns its
// canonical path.
func tempFile(t *testing.T) string {
	t.Helper()
	path := syspath.FromNative(t.TempDir()) + "/file"
	if err := os.WriteFile(path, []byte("evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// mayHoldTimes opens path through opener and reports whether the handle it
// returns holds the right to set the file's times: SetFileTime with the
// hold value is refused with access denied on a handle without it.
func mayHoldTimes(t *testing.T, opener *Opener, path string) bool {
	t.Helper()
	file, err := opener.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	err = HoldTimes(windows.Handle(file.Fd()), false)
	return !errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// requireLocalNTFS skips the test unless path is on a local NTFS volume.
func requireLocalNTFS(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(syspath.ToNative(path))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if fileSystem := localFileSystem(windows.Handle(file.Fd())); fileSystem != "NTFS" {
		t.Skipf("%s is on %q, not a local NTFS volume", path, fileSystem)
	}
}

func TestOpenerDecision(t *testing.T) {
	cases := []struct {
		name       string
		opener     *Opener
		wantJudges bool
		wantAccess []uint32
	}{
		{"a nil opener holds", nil, false, holding},
		{"the default opener holds", NewOpener(false, false), false, holding},
		{"--no-preserve-atime reads only", NewOpener(true, false), false, readingOnly},
		{"--no-preserve-atime with updates off reads only", NewOpener(true, true), false, readingOnly},
		{"updates off judges the volume, then holds", NewOpener(false, true), true, holding},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			judges, access := test.opener.judgesVolume(), accessList(test.opener.asksForRight())
			if judges != test.wantJudges || fmt.Sprint(access) != fmt.Sprint(test.wantAccess) {
				t.Errorf("judges %v, access %#x; want %v, %#x", judges, access,
					test.wantJudges, test.wantAccess)
			}
		})
	}
}

// The access lists openHolding tries.
var (
	holding     = []uint32{windows.GENERIC_READ | windows.FILE_WRITE_ATTRIBUTES, windows.GENERIC_READ}
	readingOnly = []uint32{windows.GENERIC_READ}
)

func TestTheHandleReturnedIsTheHandleJudged(t *testing.T) {
	local := tempFile(t)
	requireLocalNTFS(t, local)
	t.Run("updates off returns a local NTFS file on a read-only handle", func(t *testing.T) {
		if mayHoldTimes(t, NewOpener(false, true), local) {
			t.Error("the handle may set the file's times")
		}
	})
	t.Run("updates on returns it on a handle that holds", func(t *testing.T) {
		if !mayHoldTimes(t, NewOpener(false, false), local) {
			t.Error("the handle may not set the file's times")
		}
	})
	t.Run("a junction in the path does not stop the judgment", func(t *testing.T) {
		parent := syspath.FromNative(t.TempDir())
		if err := os.Mkdir(parent+"/target", 0o755); err != nil {
			t.Fatal(err)
		}
		makeJunction(t, syspath.ToNative(parent+"/target"), syspath.ToNative(parent+"/junction"))
		if err := os.WriteFile(parent+"/target/file", []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if mayHoldTimes(t, NewOpener(false, true), parent+"/junction/file") {
			t.Error("a local NTFS file reached through a junction got the right")
		}
	})
}

// throughShare returns the canonical path of the local path reached through
// the administrative share of its drive on this machine (C:/x as
// //localhost/C$/x), a remote volume to the opener whose server reports
// NTFS, or skips the test when the share cannot be reached.
func throughShare(t *testing.T, path string) string {
	t.Helper()
	if len(path) < 3 || path[1] != ':' {
		t.Skipf("%s is not on a drive letter", path)
	}
	shared := "//localhost/" + path[:1] + "$" + path[2:]
	if _, err := os.Lstat(syspath.ToNative(shared)); err != nil {
		t.Skipf("the administrative share is unreachable here: %v", err)
	}
	return shared
}

func TestUpdatesOffStillHoldsAFileOnARemoteVolume(t *testing.T) {
	// A file the test owns, so the right is granted wherever it is asked
	// for, read through the loopback share.
	shared := throughShare(t, tempFile(t))
	if !mayHoldTimes(t, NewOpener(false, true), shared) {
		t.Errorf("%s: opened read-only; a remote volume is not governed by the local setting", shared)
	}
}

func TestUpdatesOffHoldsAFileThroughALinkToARemoteVolume(t *testing.T) {
	file := tempFile(t)
	shared := throughShare(t, file)
	link := syspath.FromNative(t.TempDir()) + "/through"
	directory := shared[:strings.LastIndexByte(shared, '/')]
	if err := os.Symlink(syspath.ToNative(directory), syspath.ToNative(link)); err != nil {
		t.Skipf("cannot create a directory symlink here (the privilege is needed): %v", err)
	}
	if !mayHoldTimes(t, NewOpener(false, true), link+"/file") {
		t.Errorf("a file reached through a symlink to a share was opened read-only")
	}
}

// denyTimeSetting adds to path's access list an entry denying the current
// user the right to change the file's attributes, the inherited entries
// still letting it read, and clears the list when the test ends so the
// temporary directory can go. The test is skipped when the entry cannot be
// set or does not refuse that right (a privilege may override it).
func denyTimeSetting(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Skipf("cannot read the process token: %v", err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.FILE_WRITE_ATTRIBUTES,
		AccessMode:        windows.DENY_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID,
			TrusteeType: windows.TRUSTEE_IS_USER, TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid)},
	}}, nil)
	if err != nil {
		t.Skipf("cannot build an access control list: %v", err)
	}
	native := syspath.ToNative(path)
	if err := windows.SetNamedSecurityInfo(native, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Skipf("cannot set an access control list here: %v", err)
	}
	t.Cleanup(func() {
		_ = windows.SetNamedSecurityInfo(native, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION, nil, nil, nil, nil)
	})
	name, err := windows.UTF16PtrFromString(native)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err == nil {
		_ = windows.CloseHandle(handle)
		t.Skip("the access list did not refuse the right to set the times (a privilege overrides it?)")
	}
}

// readThrough reads the whole of path through opener, recording what the
// opener reports, and returns the reports as "path: reason" lines.
func readThrough(t *testing.T, opener *Opener, path string) []string {
	t.Helper()
	var reports []string
	opener.Report = func(reported, reason string) {
		reports = append(reports, reported+": "+reason)
	}
	file, err := opener.OpenRegular(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if content, err := io.ReadAll(file); err != nil || string(content) != "evidence" {
		t.Fatalf("read %q, %v; want the file's contents", content, err)
	}
	return reports
}

// accessTimeOf reads path's access time through a handle opened for its
// attributes alone, which records no access of its own.
func accessTimeOf(t *testing.T, path string) uint64 {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	accessed, err := accessTimeOn(handle)
	if err != nil {
		t.Fatal(err)
	}
	return accessed
}

func TestAFileWhoseRightToHoldIsRefusedIsReportedOnce(t *testing.T) {
	path := tempFile(t)
	denyTimeSetting(t, path)
	reports := readThrough(t, NewOpener(false, false), path)
	if want := path + ": " + reasonRightRefused; len(reports) != 1 || reports[0] != want {
		t.Errorf("reports %q, want exactly %q", reports, want)
	}
}

func TestAnOrdinaryFileIsHeldUnreported(t *testing.T) {
	path := tempFile(t)
	if err := os.Chtimes(path, time.Unix(1_600_000_000, 0), time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatal(err)
	}
	before := accessTimeOf(t, path)
	if reports := readThrough(t, NewOpener(false, false), path); len(reports) != 0 {
		t.Errorf("reports %q, want none", reports)
	}
	if got := accessTimeOf(t, path); got != before {
		t.Errorf("access time %d after the read, want %d", got, before)
	}
}

func TestNoPreserveAtimeReportsNothing(t *testing.T) {
	path := tempFile(t)
	denyTimeSetting(t, path)
	if reports := readThrough(t, NewOpener(true, false), path); len(reports) != 0 {
		t.Errorf("reports %q, want none: no hold was wanted", reports)
	}
}

// basicTimesOf returns the creation, access, write and change times of
// path (FILE_BASIC_INFO), the change time being the last change to the
// file's record, read through a handle opened for attributes alone.
func basicTimesOf(t *testing.T, path string) [4]uint64 {
	t.Helper()
	handle := attributeHandle(t, path, windows.FILE_READ_ATTRIBUTES)
	defer windows.CloseHandle(handle)
	var basic struct {
		times      [4]uint64
		attributes uint32
		_          uint32
	}
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic))); err != nil {
		t.Fatal(err)
	}
	return basic.times
}

// attributeHandle opens path for access alone, through its own reparse
// point.
func attributeHandle(t *testing.T, path string, access uint32) windows.Handle {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, access, shareAll, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

// fileUSN returns the number of the last change-journal record written for
// path (FSCTL_READ_FILE_USN_DATA), or the error that says the volume keeps
// no journal. The record is USN_RECORD_V2 (Usn at byte 24) or V3 (Usn at
// byte 40), told apart by MajorVersion at byte 4.
func fileUSN(t *testing.T, path string) (uint64, error) {
	t.Helper()
	handle := attributeHandle(t, path, windows.FILE_READ_ATTRIBUTES)
	defer windows.CloseHandle(handle)
	versions := [2]uint16{2, 3}
	record := make([]byte, 1024)
	var returned uint32
	err := windows.DeviceIoControl(handle, windows.FSCTL_READ_FILE_USN_DATA,
		(*byte)(unsafe.Pointer(&versions[0])), uint32(unsafe.Sizeof(versions)),
		&record[0], uint32(len(record)), &returned, nil)
	if err != nil {
		return 0, err
	}
	if binary.LittleEndian.Uint16(record[4:]) == 3 {
		return binary.LittleEndian.Uint64(record[40:]), nil
	}
	return binary.LittleEndian.Uint64(record[24:]), nil
}

func TestSettingTheAccessTimeLeavesTheCreationAndWriteTimes(t *testing.T) {
	path := tempFile(t)
	before := basicTimesOf(t, path)
	usnBefore, usnErr := fileUSN(t, path)
	handle := attributeHandle(t, path, windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES)
	err := setAccessTime(handle, before[1])
	windows.CloseHandle(handle)
	if err != nil {
		t.Fatal(err)
	}
	after := basicTimesOf(t, path)
	usnAfter, _ := fileUSN(t, path)
	t.Logf("change time %d before, %d after (moved %v); journal record %d before, %d after "+
		"(journal error %v)", before[3], after[3], after[3] != before[3], usnBefore, usnAfter, usnErr)
	if after[0] != before[0] || after[2] != before[2] || after[1] != before[1] {
		t.Errorf("creation, access, write times %v after setting the access time, want %v",
			after[:3], before[:3])
	}
}
