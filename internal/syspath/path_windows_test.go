package syspath

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsComparisonsIgnoreCase(t *testing.T) {
	cases := []struct {
		name string
		got  bool
		want bool
	}{
		{"paths differing in case are equal", Equal("C:/Users/Alice", "c:/users/alice"), true},
		{"different paths are not equal", Equal("C:/Users/a", "C:/Users/b"), false},
		{"a prefix matches ignoring case", HasPrefix("C:/Users/Alice/x", "c:/users/alice/"), true},
		{"a longer prefix does not match", HasPrefix("C:/a", "C:/a/b"), false},
		{"a backslashed drive path is absolute", IsAbs(`C:\x`), true},
		{"a path on the current drive is not absolute", IsAbs("/x"), false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.got != testCase.want {
				t.Fatalf("got %t, want %t", testCase.got, testCase.want)
			}
		})
	}
}

func TestWindowsCutPrefixKeepsTheSpelling(t *testing.T) {
	rest, found := CutPrefix("D:/Img/Users/Alice", "d:/img")
	if rest != "/Users/Alice" || !found {
		t.Fatalf("CutPrefix = %s, %t", rest, found)
	}
}

func TestWindowsSystemBaseReadsSystemDrive(t *testing.T) {
	t.Setenv("SystemDrive", `E:\`)
	if got := SystemBase(""); got != "E:" {
		t.Fatalf("SystemBase(\"\") = %s, want E:", got)
	}
	if got := SystemBase("D:/img"); got != "D:/img" {
		t.Fatalf("SystemBase(root) = %s, want the root", got)
	}
}

func TestWindowsExtendedPathsOpenTheFile(t *testing.T) {
	directory := FromNative(t.TempDir())
	deep := directory + "/" + strings.Repeat("d", 120) + "/" + strings.Repeat("e", 120)
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	file := deep + "/f.txt"
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, path string }{
		{"a canonical path past MAX_PATH", file},
		{"a path with . and .. segments", deep + "/./../" + strings.Repeat("e", 120) + "/f.txt"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			extended := Extended(testCase.path)
			if !strings.HasPrefix(extended, `\\?\`) {
				t.Fatalf("Extended(%s) = %s, want the prefix", testCase.path, extended)
			}
			if _, err := os.Lstat(extended); err != nil {
				t.Fatalf("Lstat(%s): %v", extended, err)
			}
		})
	}
}

func TestWindowsFromNativeTakesWhatTheOSGives(t *testing.T) {
	native := t.TempDir()
	canonical := FromNative(native)
	if strings.Contains(canonical, `\`) || !IsAbs(canonical) {
		t.Fatalf("FromNative(%s) = %s, want an absolute forward-slash path", native, canonical)
	}
	if got := ToNative(canonical); got != filepath.Clean(native) {
		t.Fatalf("ToNative(%s) = %s, want %s", canonical, got, filepath.Clean(native))
	}
}

func TestWindowsNativeErrorsRewritesAnOpenFailure(t *testing.T) {
	missing := FromNative(t.TempDir()) + "/missing"
	_, err := os.Open(missing)
	rewritten := NativeErrors(err)
	var pathError *fs.PathError
	if !errors.As(rewritten, &pathError) || pathError.Path != ToNative(missing) {
		t.Fatalf("NativeErrors(%v) = %v, want the native path", err, rewritten)
	}
	if !errors.Is(rewritten, fs.ErrNotExist) {
		t.Fatalf("NativeErrors(%v) no longer matches fs.ErrNotExist", err)
	}
}
