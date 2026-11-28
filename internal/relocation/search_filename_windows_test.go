package relocation

import (
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// errorInvalidName is ERROR_INVALID_NAME, which Windows returns for a name
// holding a character such as *, ? or a newline.
const errorInvalidName syscall.Errno = 123

// invalidFileName reports whether err says the file name's syntax is not
// valid on this platform.
func invalidFileName(err error) bool {
	return errors.Is(err, errorInvalidName)
}

// newlineFixture returns no files: Windows refuses a newline in a name.
func newlineFixture(string) map[string]string { return nil }

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
// mount-point reparse data mklink /J would write. Both paths are absolute
// and native.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(link, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
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

// makeLink makes path a symlink to target, or, where the account may not
// create symlinks (the CI runner), a junction when target is a directory. A
// link to a file is then left out and logged: the search never lists a
// symlink, so its absence gives the same result.
func makeLink(t *testing.T, target, path string) {
	t.Helper()
	err := os.Symlink(target, path)
	if !errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	if info, statErr := os.Stat(target); statErr != nil || !info.IsDir() {
		t.Logf("symlink %s left out, creating one needs a privilege: %v", path, err)
		return
	}
	makeJunction(t, target, path)
}

// linkOut makes link a junction to the directory target, both native: the
// runner may not create symlinks, and the root check treats a junction as a
// symlinked directory.
func linkOut(t *testing.T, target, link string) {
	t.Helper()
	makeJunction(t, target, link)
}
