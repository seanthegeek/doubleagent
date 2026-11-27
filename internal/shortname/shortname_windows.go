package shortname

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// pathName returns what convert, the Windows call named name, gives for the
// native path.
func pathName(test testing.TB, name string,
	convert func(path, buffer *uint16, size uint32) (uint32, error), path string) string {
	test.Helper()
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		test.Fatal(err)
	}
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	n, err := convert(pointer, &buffer[0], uint32(len(buffer)))
	if err != nil || int(n) >= len(buffer) {
		test.Fatalf("%s(%q) = %d, %v", name, path, n, err)
	}
	return windows.UTF16ToString(buffer[:n])
}

// ShortPath returns the native path Windows gives through GetShortPathName for the
// existing native path long: every component that has an 8.3 short name
// spelled by it, the rest as they are.
func ShortPath(test testing.TB, long string) string {
	test.Helper()
	return pathName(test, "GetShortPathName", windows.GetShortPathName, long)
}

// Dir makes a directory with a long name under a new temporary directory and
// returns it in native form twice: short, with the short name of every
// component that has one, as GetShortPathName gives it, and long, with every
// component's long name, as GetLongPathName gives it. The test is skipped
// when the two are the same: the volume then makes no short names.
func Dir(test testing.TB) (short, long string) {
	test.Helper()
	made := filepath.Join(test.TempDir(), "a long directory name.d")
	if err := osfile.MkdirAll(syspath.FromNative(made), 0o755); err != nil {
		test.Fatal(err)
	}
	short = ShortPath(test, made)
	long = pathName(test, "GetLongPathName", windows.GetLongPathName, short)
	if short == long {
		test.Skipf("the volume of %s makes no 8.3 short names", long)
	}
	return short, long
}
