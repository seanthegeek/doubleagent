package fsmeta

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// markCaseSensitiveByCall marks the empty directory dir case-sensitive
// through SetFileInformationByHandle.
func markCaseSensitiveByCall(dir string) error {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES|windows.FILE_READ_ATTRIBUTES,
		shareAll, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	flags := uint32(windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR)
	return windows.SetFileInformationByHandle(handle, windows.FileCaseSensitiveInfo,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
}

func TestMeasureCaseSensitiveDirectories(t *testing.T) {
	root := t.TempDir()
	byCall, byProgram := filepath.Join(root, "bycall"), filepath.Join(root, "byprogram")
	for _, dir := range []string{byCall, byProgram} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var report []string
	callErr := markCaseSensitiveByCall(byCall)
	report = append(report, fmt.Sprintf("SetFileInformationByHandle: %v; caseSensitive=%v",
		callErr, caseSensitive(syspath.FromNative(byCall))))
	output, programErr := exec.Command("fsutil.exe", "file", "setCaseSensitiveInfo", byProgram,
		"enable").CombinedOutput()
	report = append(report, fmt.Sprintf("fsutil: %v %q; caseSensitive=%v", programErr,
		strings.TrimSpace(string(output)), caseSensitive(syspath.FromNative(byProgram))))
	dir := syspath.FromNative(byCall)
	if callErr != nil {
		dir = syspath.FromNative(byProgram)
	}
	for _, name := range []string{"Alice", "alice"} {
		err := os.Mkdir(syspath.ToNative(dir+"/"+name), 0o755)
		report = append(report, fmt.Sprintf("mkdir %s: %v", name, err))
	}
	entries, err := os.ReadDir(syspath.ToNative(dir))
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	report = append(report, fmt.Sprintf("listing %v %v", names, err))
	for _, name := range []string{"Alice", "alice", "ALICE"} {
		found, ok := OnDiskName(dir, name)
		meta, err := Lstat(dir + "/" + name)
		report = append(report, fmt.Sprintf("OnDiskName(%s)=%q,%v Lstat dev=%d ino=%d kind=%d err=%v",
			name, found, ok, meta.Dev, meta.Ino, meta.Kind, err))
		report = append(report, fmt.Sprintf("OnDiskPath(%s/x)=%q", name,
			strings.TrimPrefix(OnDiskPath("", dir+"/"+name+"/x"), dir)))
	}
	t.Errorf("measurement:\n%s", strings.Join(report, "\n"))
}
