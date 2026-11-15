package fsmeta

import (
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// mkdirExtended creates each directory below base, a canonical path, through
// the \\?\ form, so names NTFS keeps apart but Win32 might not are made as
// written.
func mkdirExtended(t *testing.T, base string, relatives ...string) {
	t.Helper()
	for _, relative := range relatives {
		if err := os.MkdirAll(syspath.Extended(base+"/"+relative), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOnDiskPathAsksTheFileSystem(t *testing.T) {
	base := OnDiskPath("", syspath.FromNative(t.TempDir()))
	mkdirExtended(t, base, "\u00c9mile/sub", "img", "\u0131mg", ".secret", ".\u017fecret")
	cases := []struct{ name, path, want string }{
		{"an accented letter in another case takes the disk's spelling",
			base + "/\u00e9mile/SUB", base + "/\u00c9mile/sub"},
		{"a missing component and what follows stay as written",
			base + "/\u00e9mile/Gone/SUB", base + "/\u00c9mile/Gone/SUB"},
		{"dot-dot stops the respelling", base + "/\u00e9mile/../IMG", base + "/\u00c9mile/../IMG"},
		{"capital I names img, never dotless-i img beside it", base + "/IMG", base + "/img"},
		{"dotless-i img keeps its own spelling", base + "/\u0131MG", base + "/\u0131mg"},
		{"long s secret is its own entry", base + "/.\u017fECRET", base + "/.\u017fecret"},
		{"a wildcard component stays as written", base + "/i*", base + "/i*"},
		{"a relative path is unchanged", "\u00e9mile/sub", "\u00e9mile/sub"},
		{"a bare drive is unchanged", "C:", "C:"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := OnDiskPath("", test.path); got != test.want {
				t.Errorf("OnDiskPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestOnDiskPathStopsAtALink(t *testing.T) {
	base := OnDiskPath("", syspath.FromNative(t.TempDir()))
	mkdirExtended(t, base, "other/Real/Inner")
	makeJunction(t, syspath.ToNative(base+"/other/Real"), syspath.ToNative(base+"/J"))
	// Inner exists behind the junction as Inner; the walk stops at J, so the
	// typed inner stays.
	if got, want := OnDiskPath("", base+"/j/inner"), base+"/J/inner"; got != want {
		t.Errorf("OnDiskPath through a junction = %q, want %q", got, want)
	}
}

func TestOnDiskPathNeverLooksAtOrAboveItsBase(t *testing.T) {
	base := OnDiskPath("", syspath.FromNative(t.TempDir()))
	mkdirExtended(t, base, "Img/Sub")
	cases := []struct{ name, under, path, want string }{
		{"the base keeps its typed spelling and what lies below it is respelled",
			base + "/img", base + "/img/sub", base + "/img/Sub"},
		{"a path beside the base is left as written", base + "/img", base + "/other/sub",
			base + "/other/sub"},
		{"the base itself is left as written", base + "/img", base + "/img", base + "/img"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := OnDiskPath(test.under, test.path); got != test.want {
				t.Errorf("OnDiskPath(%q, %q) = %q, want %q", test.under, test.path, got, test.want)
			}
		})
	}
}

func TestOnDiskPathGivesTheLongNameOfAShortOne(t *testing.T) {
	temp := os.Getenv("TEMP")
	if !strings.Contains(temp, "~") {
		t.Skipf("TEMP %q holds no short 8.3 name to respell", temp)
	}
	got := OnDiskPath("", syspath.FromNative(temp))
	want := syspath.FromNative(longPathName(t, temp))
	if strings.Contains(got, "~") || !syspath.Equal(got, want) {
		t.Errorf("OnDiskPath(%q) = %q, want the long form %q", temp, got, want)
	}
	// A catalog literal still never matches through a short name.
	short := shortComponent(temp)
	parent := syspath.FromNative(temp[:strings.Index(temp, `\`+short)])
	if name, ok := OnDiskName(parent, short); ok {
		t.Errorf("OnDiskName respelled the short name %q to %q, want it refused", short, name)
	}
}

// shortComponent returns the first component of a native path holding ~.
func shortComponent(path string) string {
	for _, component := range strings.Split(path, `\`) {
		if strings.Contains(component, "~") {
			return component
		}
	}
	return ""
}

// longPathName returns the long form of a native path, as Windows gives it.
func longPathName(t *testing.T, path string) string {
	t.Helper()
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]uint16, 4096)
	n, err := windows.GetLongPathName(pointer, &buffer[0], uint32(len(buffer)))
	if err != nil || int(n) >= len(buffer) {
		t.Fatalf("GetLongPathName(%q): %v", path, err)
	}
	return windows.UTF16ToString(buffer[:n])
}

// caseSensitiveDir makes a new empty directory under base whose names are
// compared with case, through FileCaseSensitiveInfo, and returns it; the test
// is skipped where Windows refuses the setting.
func caseSensitiveDir(t *testing.T, base string) string {
	t.Helper()
	dir := base + "/cs"
	mkdirExtended(t, base, "cs")
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir))
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	flags := uint32(windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR)
	err = windows.SetFileInformationByHandle(handle, windows.FileCaseSensitiveInfo,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
	if err != nil {
		t.Skipf("a case-sensitive directory needs FileCaseSensitiveInfo, which Windows refused here: %v", err)
	}
	return dir
}

func TestOnDiskPathInACaseSensitiveDirectory(t *testing.T) {
	dir := caseSensitiveDir(t, OnDiskPath("", syspath.FromNative(t.TempDir())))
	mkdirExtended(t, dir, "FOO", "Both", "both")
	cases := []struct{ name, path, want string }{
		{"a name spelled otherwise than the lone entry stays as typed", dir + "/foo", dir + "/foo"},
		{"a name spelled as the entry is kept", dir + "/FOO", dir + "/FOO"},
		{"of two entries differing in case the typed one is named", dir + "/both", dir + "/both"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := OnDiskPath("", test.path); got != test.want {
				t.Errorf("OnDiskPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestCaseSensitiveReadsTheDirectorysMark(t *testing.T) {
	base := OnDiskPath("", syspath.FromNative(t.TempDir()))
	mkdirExtended(t, base, "plain")
	if caseSensitive(base + "/plain") {
		t.Error("caseSensitive of a plain directory = true, want false")
	}
	if dir := caseSensitiveDir(t, base); !caseSensitive(dir) {
		t.Errorf("caseSensitive(%q) = false for a directory marked case-sensitive", dir)
	}
}
