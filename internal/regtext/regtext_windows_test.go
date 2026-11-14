package regtext

import (
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// testKey creates a throwaway key under HKEY_CURRENT_USER\Software, deleted
// when the test ends.
func testKey(t *testing.T) registry.Key {
	t.Helper()
	subkey := `Software\doubleagent-test-` + rand.Text()
	key, _, err := registry.CreateKey(registry.CURRENT_USER, subkey,
		registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("create HKCU\\%s: %v", subkey, err)
	}
	t.Cleanup(func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, subkey); err != nil {
			t.Errorf("delete HKCU\\%s: %v", subkey, err)
		}
	})
	return key
}

func TestRead(t *testing.T) {
	key := testKey(t)
	long := strings.Repeat("x", 1000)
	writes := []error{
		key.SetStringValue("empty", ""),
		key.SetStringValue("long", long),
		key.SetExpandStringValue("expand", `%SystemRoot%\x`),
		key.SetBinaryValue("binary", make([]byte, 1<<20)),
		key.SetDWordValue("number", 7),
	}
	if err := errors.Join(writes...); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, value, want string
		kind              uint32
		err               error
	}{
		{"an empty string is empty text", "empty", "", registry.SZ, nil},
		{"a string longer than one small buffer is read whole", "long", long, registry.SZ, nil},
		{"an expandable string is never expanded", "expand", `%SystemRoot%\x`,
			registry.EXPAND_SZ, nil},
		{"a large binary value is refused with its type", "binary", "", registry.BINARY,
			registry.ErrUnexpectedType},
		{"a number is refused with its type", "number", "", registry.DWORD,
			registry.ErrUnexpectedType},
		{"a missing value is not found", "missing", "", 0, registry.ErrNotExist},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			text, kind, err := Read(key, test.value)
			if text != test.want || !errors.Is(err, test.err) ||
				(test.err != registry.ErrNotExist && kind != test.kind) {
				t.Errorf("Read(%s) = %d bytes, type %d, %v; want %d bytes, type %d, %v",
					test.value, len(text), kind, err, len(test.want), test.kind, test.err)
			}
		})
	}
}
