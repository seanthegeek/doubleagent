package input

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// makeJunction makes link a junction to the directory target without
// running a program: it creates an empty directory and sets on it the
// mount-point reparse data mklink /J would write. Both paths are absolute.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(link, 0o755); err != nil {
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

func TestAJunctionInsideADirectoryCollectionIsNeverPassedThrough(t *testing.T) {
	dir := t.TempDir()
	outside := plantOutside(t, dir, `tmp\slug\shell_history`)
	root := filepath.Join(dir, "collection")
	writeManifest(t, root, collectedRow("a", "/home/a", "gemini-cli",
		"/home/a/.gemini/tmp/slug/shell_history", "fs/home/a/.gemini/tmp/slug/shell_history"))
	makeJunction(t, outside, filepath.Join(root, "fs", "home", "a", ".gemini"))
	collection, err := Open(t.Context(), root, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantConfined(t, collection)
	wantProblem(t, collection, "shell_history: through a symbolic link, not followed")
	if len(collection.Artifacts) != 0 {
		t.Errorf("artifacts %d read through the junction", len(collection.Artifacts))
	}
	if path, ok := collection.Homes[0].PathOf(".gemini/tmp/slug/shell_history"); ok {
		t.Errorf("PathOf passed through the junction to %s", path)
	}
}

func TestANameWithAColonIsNotOneComponentOnWindows(t *testing.T) {
	home := &Home{DiskPath: t.TempDir()}
	for _, rel := range []string{"a:b", "x/C:", "f.txt:stream"} {
		if path, ok := home.PathOf(rel); ok {
			t.Errorf("PathOf(%q) = %s", rel, path)
		}
	}
	if err := checkElement("2026-01-01T10:00:00.json"); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Errorf("checkElement of a name with a colon: %v", err)
	}
}
