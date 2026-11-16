package volumes

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

func TestUnmatchedVolumeRowCarriesTheDataDirectoryAttributes(t *testing.T) {
	fixture := newSetup(t, false)
	fixture.write(t, "var/lib/docker/volumes/postgres/_data/PG_VERSION", "16")
	var notes []string
	if _, err := Collect(context.Background(), fixture.collector, fixture.bases(), nil, &notes); err != nil {
		t.Fatal(err)
	}
	meta, err := fsmeta.Lstat(fixture.root + "/var/lib/docker/volumes/postgres/_data")
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(fixture.rows.Bytes(), &row); err != nil {
		t.Fatalf("row %s: %v", fixture.rows, err)
	}
	want := fsmeta.AttributesString(meta.Attributes)
	if row["attributes"] != want || row["owner"] != "" || row["mode"] != "" {
		t.Errorf("row %s, want attributes %q, owner and mode empty", fixture.rows, want)
	}
}

func TestDataJunctionOutOfTheRootGivesOneOutcome(t *testing.T) {
	logs := map[string]string{}
	for _, readable := range []bool{true, false} {
		fixture := newSetup(t, false)
		fixture.mkdir(t, "var/lib/docker/volumes/v1")
		outside := syspath.FromNative(t.TempDir())
		fixture.collector.RootCheck = walk.NewRootCheck(fixture.root)
		if err := os.WriteFile(syspath.ToNative(outside+"/x"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if !readable {
			denyEveryone(t, outside)
			requireUnlistable(t, outside)
		}
		makeJunction(t, outside, fixture.root+"/var/lib/docker/volumes/v1/_data")
		var notes []string
		if _, err := Collect(context.Background(), fixture.collector, fixture.bases(), nil,
			&notes); err != nil {
			t.Fatal(err)
		}
		name := "readable"
		if !readable {
			name = "unreadable"
		}
		logs[name] = strings.ReplaceAll(strings.Join(fixture.log.lines, "\n"),
			syspath.ToNative(fixture.root), "{ROOT}")
		if len(notes) != 0 || fixture.rows.Len() != 0 {
			t.Errorf("notes %q, rows %s; want none", notes, fixture.rows)
		}
	}
	if logs["readable"] != logs["unreadable"] ||
		!strings.Contains(logs["readable"], "volume v1 has no _data directory, skipped") {
		t.Errorf("logs differ or lack the skip line:\n%s\nversus\n%s", logs["readable"],
			logs["unreadable"])
	}
}

// denyEveryone makes dir unlistable but still visible: a protected list that
// denies listing its entries (FILE_LIST_DIRECTORY) and allows the rest, so a
// stat through a junction succeeds and only the listing fails. The list is
// opened again when the test ends.
func denyEveryone(t *testing.T, dir string) {
	t.Helper()
	set := func(sddl string) error {
		descriptor, err := windows.SecurityDescriptorFromString(sddl)
		if err != nil {
			return err
		}
		dacl, _, err := descriptor.DACL()
		if err != nil {
			return err
		}
		return windows.SetNamedSecurityInfo(syspath.ToNative(dir), windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil,
			dacl, nil)
	}
	if err := set("D:P(D;;0x1;;;WD)(A;OICI;FA;;;WD)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := set("D:P(A;OICI;GA;;;WD)"); err != nil {
			t.Errorf("give back access to %s: %v", dir, err)
		}
	})
}

// requireUnlistable stops the test unless dir, after denyEveryone, cannot be
// listed: otherwise the unreadable case is the readable one again and proves
// nothing. An elevated token whose rights pass the deny entry is a runner
// fact, so the test skips there; anything else is a broken fixture.
func requireUnlistable(t *testing.T, dir string) {
	t.Helper()
	if _, failed, _ := fsmeta.DirReadError(nil, dir); failed {
		return
	}
	if windows.GetCurrentProcessToken().IsElevated() {
		t.Skipf("%s is still listable after its deny entry under this elevated token", dir)
	}
	t.Fatalf("%s is still listable after its deny entry", dir)
}

// makeJunction makes link a junction to the directory target, both
// canonical, by setting on a new empty directory the mount-point reparse
// data mklink /J writes.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(syspath.ToNative(link), 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(syspath.ToNative(link))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	native := syspath.ToNative(target)
	data := mountPointData(`\??\`+native+`\`, native)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if err != nil {
		t.Fatal(err)
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
