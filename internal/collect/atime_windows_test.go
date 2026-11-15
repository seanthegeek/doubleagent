package collect

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// accessTimeOf returns the last-access time of path, read through a handle
// opened for attributes alone, which records no access of its own.
func accessTimeOf(t *testing.T, path string) windows.Filetime {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		t.Fatal(err)
	}
	return data.LastAccessTime
}

// requireNTFS skips the test unless dir lies on an NTFS volume, the file
// system the registry setting governs.
func requireNTFS(t *testing.T, dir string) {
	t.Helper()
	volume := filepath.VolumeName(dir) + `\`
	root, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		t.Fatal(err)
	}
	fileSystem := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &fileSystem[0],
		uint32(len(fileSystem))); err != nil {
		t.Skipf("cannot read the file system of %s: %v", volume, err)
	}
	if name := windows.UTF16ToString(fileSystem); name != "NTFS" {
		t.Skipf("the temporary directory is on %s, not NTFS", name)
	}
}

// measuredAccessUpdates reads a scratch file in dir through a plain
// read-only open after setting its access time well into the past, and
// reports whether the read moved it. Microsoft's fsutil behavior page notes
// that a file's access time as queried is current even while the disk's
// copy lags.
func measuredAccessUpdates(t *testing.T, dir string) accessUpdates {
	t.Helper()
	probe := filepath.Join(dir, "access-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(probe, time.Unix(1_600_000_000, 0), time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatal(err)
	}
	before := accessTimeOf(t, probe)
	if _, err := os.ReadFile(probe); err != nil {
		t.Fatal(err)
	}
	if accessTimeOf(t, probe) == before {
		return accessOff
	}
	return accessOn
}

func TestRegistrySettingMatchesWhatAReadMeasures(t *testing.T) {
	dir := t.TempDir()
	requireNTFS(t, dir)
	state, raw, read := readLastAccessSetting()
	t.Logf("NtfsDisableLastAccessUpdate: read %v, raw value %#x, state %d", read, raw, state)
	if state == accessUnknown {
		t.Skipf("the setting could not be interpreted (read %v, raw value %#x)", read, raw)
	}
	if measured := measuredAccessUpdates(t, dir); measured != state {
		t.Errorf("the registry value %#x reads as %s, but a read measured %s",
			raw, stateName(state), stateName(measured))
	}
}

// stateName names a state for a failure message.
func stateName(state accessUpdates) string {
	switch state {
	case accessOn:
		return "updates on"
	case accessOff:
		return "updates off"
	}
	return "unknown"
}
