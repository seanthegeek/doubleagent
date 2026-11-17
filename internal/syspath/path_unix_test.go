//go:build unix

package syspath

import (
	"io/fs"
	"syscall"
	"testing"
)

func TestUnixConversionsAreIdentities(t *testing.T) {
	conversions := map[string]func(string) string{
		"FromNative": FromNative, "ToNative": ToNative, "Extended": Extended,
		"Fold": Fold, "ArchiveRel": ArchiveRel, "SystemBase": SystemBase,
	}
	inputs := []string{"", "/", "/Users/Alice/", `C:\x`, "rel/../x", "/a\xff"}
	for name, convert := range conversions {
		for _, input := range inputs {
			t.Run(name+" keeps "+input, func(t *testing.T) {
				if got := convert(input); got != input {
					t.Fatalf("%s(%s) = %s", name, input, got)
				}
			})
		}
	}
}

func TestUnixComparisonsKeepCase(t *testing.T) {
	cases := []struct {
		name string
		got  bool
		want bool
	}{
		{"equal bytes are equal", Equal("/a/B", "/a/B"), true},
		{"a case difference is a different path", Equal("/a/B", "/a/b"), false},
		{"a prefix matches byte for byte", HasPrefix("/home/alice/x", "/home/alice/"), true},
		{"a prefix differing in case does not match", HasPrefix("/Home/alice", "/home/"), false},
		{"a path is absolute with a leading slash", IsAbs("/x"), true},
		{"a drive path is not absolute on Unix", IsAbs("C:/x"), false},
		{"a relative path is not absolute", IsAbs("x"), false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.got != testCase.want {
				t.Fatalf("got %t, want %t", testCase.got, testCase.want)
			}
		})
	}
}

func TestUnixCutPrefix(t *testing.T) {
	if rest, found := CutPrefix("/img/Users/a", "/img"); rest != "/Users/a" || !found {
		t.Fatalf("CutPrefix = %s, %t", rest, found)
	}
	if rest, found := CutPrefix("/IMG/Users/a", "/img"); rest != "/IMG/Users/a" || found {
		t.Fatalf("CutPrefix ignoring case = %s, %t", rest, found)
	}
}

func TestUnixNativeErrorsReturnsTheSameError(t *testing.T) {
	input := &fs.PathError{Op: "open", Path: "/a/b", Err: syscall.ENOENT}
	if got := NativeErrors(input); got != error(input) {
		t.Fatalf("NativeErrors() = %v, want the same error", got)
	}
}
