package projects

import (
	"os"
	"reflect"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// caseSensitiveDir makes a new directory under a temporary directory, marks
// it case-sensitive, as WSL marks the directories it makes, and returns it;
// the test is skipped where Windows refuses.
func caseSensitiveDir(t *testing.T) string {
	t.Helper()
	dir := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir())) + "/cs"
	if err := os.Mkdir(syspath.ToNative(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir))
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
		t.Skipf("cannot mark %s case-sensitive: %v", dir, err)
	}
	return dir
}

func TestAStaleSpellingDoesNotSwallowTheRealProject(t *testing.T) {
	home := caseSensitiveDir(t)
	if err := os.MkdirAll(syspath.ToNative(home+"/src/foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	same := follow.SameEntry(nil)
	// A config names the renamed Foo before the real foo.
	got := keepProjectDirs(mergeProjectOwners([]Project{{Path: home + "/src/Foo"},
		{Path: home + "/src/foo"}}, same), nil, nil, same)
	if want := []Project{{Path: home + "/src/foo"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("projects %q, want %q: the missing Foo merged the real foo away", got, want)
	}
}
