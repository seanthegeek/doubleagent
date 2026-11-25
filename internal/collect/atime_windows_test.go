package collect

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/syspath"
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

func TestADefaultRunLeavesTheAccessTimeOfEveryFileItReads(t *testing.T) {
	files := map[string]string{
		// Read by project discovery and copied as a catalog file.
		"Users/alice/.claude.json": "{\"projects\":{}}\n",
		// Read by the startup-file search, then copied for its hint.
		"Users/alice/.bashrc": "export CLAUDE_CONFIG_DIR=/home/alice/cc\n",
		// Read only by the --env-scan-home sweep, then copied for its hint.
		"Users/alice/notes/env.txt": "CODEX_HOME=/home/alice/cx\n",
	}
	root := writeImage(t, files, nil)
	requireNTFS(t, root)
	if measuredAccessUpdates(t, t.TempDir()) == accessOff {
		t.Skip("last-access updates are off on this volume: a read did not move the access time")
	}
	accessed, written := time.Unix(1_600_000_000, 0), time.Unix(1_700_000_000, 0)
	before := map[string]windows.Filetime{}
	for path := range files {
		if err := os.Chtimes(root+"/"+path, accessed, written); err != nil {
			t.Fatal(err)
		}
		before[path] = accessTimeOf(t, root+"/"+path)
	}
	outDir := t.TempDir()
	options := testOptions(root, outDir)
	options.EnvScanHome = true
	var stdout, stderr bytes.Buffer
	if code := runCollection(context.Background(), options, testHost(t), &stdout,
		&stderr); code != ExitOK {
		t.Fatalf("exit code %d, stderr %q", code, stderr.String())
	}
	for path := range files {
		manifestRow(t, outDir, root+"/"+path)
		if got := accessTimeOf(t, root+"/"+path); got != before[path] {
			t.Errorf("%s: access time %v after the run, want %v", path, got, before[path])
		}
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

func TestARunLogsEachFileReadWithoutTheHold(t *testing.T) {
	const denied = "Users/alice/.claude/settings.json"
	cases := []struct {
		name     string
		deny     bool
		wantLine bool
	}{
		{"a file whose right to hold is refused is logged and counted", true, true},
		{"a clean image logs neither line", false, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeImage(t, map[string]string{denied: "{}\n",
				"Users/alice/.claude/CLAUDE.md": "notes\n"}, nil)
			if test.deny {
				denyTimeSetting(t, root+"/"+denied)
			}
			outDir := t.TempDir()
			var stdout, stderr bytes.Buffer
			if code := runCollection(context.Background(), testOptions(root, outDir), testHost(t),
				&stdout, &stderr); code != ExitOK {
				t.Fatalf("exit code %d, stderr %q", code, stderr.String())
			}
			manifestRow(t, outDir, root+"/"+denied)
			log := runOutput(t, outDir, ".log")
			pathLine := " access time not held: " + syspath.ToNative(root+"/"+denied) +
				": right to hold refused\n"
			countLine := " access times not held: 1 file(s)\n"
			if test.wantLine && (strings.Count(log, pathLine) != 1 || strings.Count(log, countLine) != 1) {
				t.Errorf("the log lacks %q or %q once each:\n%s", pathLine, countLine, log)
			}
			if !test.wantLine && strings.Contains(log, "not held") {
				t.Errorf("the log of a clean image reports a file not held:\n%s", log)
			}
		})
	}
}
