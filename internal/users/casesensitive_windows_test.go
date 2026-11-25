package users

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// caseSensitiveDir makes the directory path, canonical, and marks it
// case-sensitive, as WSL marks the directories it creates, so two entries
// whose names differ only in case can live in it. The test is skipped where
// Windows refuses the mark.
func caseSensitiveDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(syspath.ToNative(path), 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, windows.FILE_SHARE_READ|
		windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	flags := uint32(windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR)
	if err := windows.SetFileInformationByHandle(handle, windows.FileCaseSensitiveInfo,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags))); err != nil {
		t.Skipf("cannot mark %s case-sensitive: %v", path, err)
	}
}

func TestTwoHomesDifferingOnlyInCaseAreBothKept(t *testing.T) {
	root := writeTree(t, nil, nil)
	caseSensitiveDir(t, root+"/Users")
	for _, user := range []string{"Alice", "alice"} {
		if err := os.Mkdir(syspath.ToNative(root+"/Users/"+user), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	report := find(t.Context(), nil, root, "", nil, nil, nil)
	var homes []string
	for _, home := range report.Homes {
		homes = append(homes, home.User+":"+strings.TrimPrefix(home.Path, root))
	}
	if want := []string{"Alice:/Users/Alice", "alice:/Users/alice"}; !reflect.DeepEqual(homes, want) {
		t.Errorf("homes = %v, want %v: the two directories are two homes", homes, want)
	}
}
