package analyze

import (
	"os"
	"testing"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/shortname"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// directoryAddingNothing makes a directory under the test's temporary one
// whose access list lets the account list, read and delete it but add
// nothing, and returns its path.
func directoryAddingNothing(t *testing.T) string {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FRSD;;;" +
		user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	attributes := &windows.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(*attributes))
	path := syspath.FromNative(t.TempDir()) + "/shut"
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CreateDirectory(name, attributes); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheWriteProbeOnWindows(t *testing.T) {
	cases := []struct {
		name string
		// dir returns the directory as the analyst may give it.
		dir  func(t *testing.T) string
		want bool
	}{
		{"a directory the account may add to is writable", func(t *testing.T) string {
			return t.TempDir()
		}, true},
		{"one given in the \\\\?\\ form is writable", func(t *testing.T) string {
			return `\\?\` + t.TempDir()
		}, true},
		{"one given with a trailing backslash is writable", func(t *testing.T) string {
			return t.TempDir() + `\`
		}, true},
		{"one given through a short name is writable", func(t *testing.T) string {
			short, _ := shortname.Dir(t)
			return short
		}, true},
		{"a directory whose access list denies adding files is not", func(t *testing.T) string {
			return syspath.ToNative(directoryAddingNothing(t))
		}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dir := test.dir(t)
			if got := probeWrite(dir); got != test.want {
				t.Errorf("probeWrite(%s) = %v, want %v", dir, got, test.want)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
				t.Errorf("the probe left %v (%v)", entries, err)
			}
		})
	}
}
