package stage

import (
	"testing"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// caseFlag reads or, with set, writes the case-sensitive flag of the
// directory dir.
func caseFlag(t *testing.T, dir string, set bool) bool {
	t.Helper()
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES,
		shareAll, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	flags := uint32(windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR)
	size := uint32(unsafe.Sizeof(flags))
	if set {
		if err := windows.SetFileInformationByHandle(handle, windows.FileCaseSensitiveInfo,
			(*byte)(unsafe.Pointer(&flags)), size); err != nil {
			t.Fatal(err)
		}
	}
	flags = 0
	if err := windows.GetFileInformationByHandleEx(handle, windows.FileCaseSensitiveInfo,
		(*byte)(unsafe.Pointer(&flags)), size); err != nil {
		t.Fatal(err)
	}
	return flags&windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR != 0
}

func TestDirectoriesMadeInACaseSensitiveDirectoryInheritTheFlag(t *testing.T) {
	dir := syspath.FromNative(t.TempDir()) + "/fs"
	if err := osfile.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if !caseFlag(t, dir, true) {
		t.Fatal("the flag did not stay set")
	}
	// The stager makes its directories through osfile.MkdirAll (makeDir).
	if err := osfile.MkdirAll(dir+"/child/grandchild", 0o777); err != nil {
		t.Fatal(err)
	}
	for _, made := range []string{dir + "/child", dir + "/child/grandchild"} {
		if !caseFlag(t, made, false) {
			t.Errorf("%s was made without the case-sensitive flag", made)
		}
	}
}
