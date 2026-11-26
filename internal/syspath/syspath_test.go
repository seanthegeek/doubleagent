package syspath

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

type conversionCase struct {
	name  string
	input string
	want  string
}

// runConversions runs convert over each case as a subtest and checks its
// result.
func runConversions(t *testing.T, convert func(string) string, cases []conversionCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := convert(testCase.input); got != testCase.want {
				t.Fatalf("convert(%s) = %s, want %s", testCase.input, got, testCase.want)
			}
		})
	}
}

func TestFromWindows(t *testing.T) {
	runConversions(t, fromWindows, []conversionCase{
		{"an extended drive path loses its prefix and trailing separator",
			`\\?\C:\Users\a\`, "C:/Users/a"},
		{"an extended UNC path becomes a double-slash path", `\\?\UNC\srv\sh\x`, "//srv/sh/x"},
		{"the UNC prefix is matched ignoring case", `\\?\unc\srv\sh`, "//srv/sh"},
		{"a drive root becomes the bare drive", `C:\`, "C:"},
		{"a bare drive stays", "C:", "C:"},
		{"a plain native path gets forward slashes", `D:\img\Users`, "D:/img/Users"},
		{"a UNC root keeps its double slash", `\\server\share\`, "//server/share"},
		{"a canonical path is unchanged", "C:/Users/alice", "C:/Users/alice"},
		{"a relative path gets forward slashes", `rel\x`, "rel/x"},
		{"a lone separator stays", `\`, "/"},
		{"only one trailing separator is trimmed", `C:\a\\`, "C:/a/"},
		{"the empty path stays empty", "", ""},
	})
}

func TestToWindows(t *testing.T) {
	runConversions(t, toWindows, []conversionCase{
		{"a bare drive gets its root separator", "C:", `C:\`},
		{"a canonical path gets backslashes", "C:/Users/alice", `C:\Users\alice`},
		{"a UNC root gets backslashes", "//server/share", `\\server\share`},
		{"a lone slash becomes a backslash", "/", `\`},
		{"a relative path gets backslashes", "rel/x", `rel\x`},
		{"the empty path stays empty", "", ""},
	})
}

func TestFromWindowsUndoesToWindows(t *testing.T) {
	for _, canonical := range []string{"C:", "C:/Users/alice", "//server/share/x", "rel/x"} {
		t.Run(canonical+" comes back unchanged", func(t *testing.T) {
			if got := fromWindows(toWindows(canonical)); got != canonical {
				t.Fatalf("fromWindows(toWindows(%s)) = %s", canonical, got)
			}
		})
	}
}

func TestExtendedWindows(t *testing.T) {
	runConversions(t, extendedWindows, []conversionCase{
		{"a canonical drive path gets the prefix", "C:/Users/a", `\\?\C:\Users\a`},
		{"a native drive path gets the prefix", `C:\Users\a`, `\\?\C:\Users\a`},
		{"a .. segment pops its parent", "C:/a/../b", `\\?\C:\b`},
		{"a .. segment never pops the drive", "C:/..", `\\?\C:\`},
		{"many .. segments stop at the drive", "C:/a/../../../b", `\\?\C:\b`},
		{"a . segment and doubled separators are dropped", "C:/a/./b//c", `\\?\C:\a\b\c`},
		{"a trailing separator is dropped", `C:\a\`, `\\?\C:\a`},
		{"a bare drive is its root", "C:", `\\?\C:\`},
		{"a drive root keeps its separator", `C:\`, `\\?\C:\`},
		{"a UNC path gets the UNC prefix", "//srv/sh/x", `\\?\UNC\srv\sh\x`},
		{"a .. segment never pops the share", "//srv/sh/../x", `\\?\UNC\srv\sh\x`},
		{"a UNC root ends in a separator", `\\srv\sh`, `\\?\UNC\srv\sh\`},
		{"an extended path is unchanged", `\\?\C:\a\..\b`, `\\?\C:\a\..\b`},
		{"a device path is unchanged", `\\.\pipe\x`, `\\.\pipe\x`},
		{"a relative path is unchanged", "rel/x", "rel/x"},
		{"a drive-relative path is unchanged", "C:rel", "C:rel"},
		{"a path rooted on the current drive is unchanged", "/x", "/x"},
		{"the empty path stays empty", "", ""},
		{"a long path gets the prefix too", "C:/" + longSegment, `\\?\C:\` + longSegment},
	})
}

const longSegment = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
	"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" +
	"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestExtendedParts(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		prefix string
		body   string
		keep   int
		ok     bool
	}{
		{"a drive path keeps one segment", "C:/a", `\\?\`, `C:\a`, 1, true},
		{"a UNC path keeps two segments", "//s/sh/a", `\\?\UNC\`, `s\sh\a`, 2, true},
		{"a relative path is not ok", "a/b", "", "", 0, false},
		{"a path that is not a drive is not ok", `1:\a`, "", "", 0, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			prefix, body, keep, ok := extendedParts(testCase.input)
			if prefix != testCase.prefix || body != testCase.body || keep != testCase.keep ||
				ok != testCase.ok {
				t.Fatalf("extendedParts(%s) = %s, %s, %d, %t", testCase.input, prefix, body, keep, ok)
			}
		})
	}
}

func TestFoldASCII(t *testing.T) {
	runConversions(t, textlines.LowerASCII, []conversionCase{
		{"ASCII capitals are lowered", "C:/Users/Alice", "c:/users/alice"},
		{"bytes beyond ASCII are kept", "C:/\xc3\x89t\xc3\xa9", "c:/\xc3\x89t\xc3\xa9"},
		{"invalid UTF-8 is kept", "A\xff", "a\xff"},
	})
}

func TestFoldASCIIKeepsApartNamesNTFSKeepsApart(t *testing.T) {
	// NTFS's case table leaves each of these unchanged, so a volume can hold
	// both names of a pair side by side; a fold that merged them would
	// collect one and lose the other.
	pairs := []struct{ name, first, second string }{
		{"dotless i and I", "\u0131mg", "IMG"},
		{"long s and S", "\u017fecret", "SECRET"},
		{"micro sign and capital mu", "\u00b5", "\u039c"},
		{"final sigma and capital sigma", "\u03c2", "\u03a3"},
		{"a pair outside the Basic Multilingual Plane", "\U00010428", "\U00010400"},
	}
	for _, pair := range pairs {
		t.Run(pair.name+" stay two keys", func(t *testing.T) {
			if textlines.LowerASCII(pair.first) == textlines.LowerASCII(pair.second) {
				t.Errorf("textlines.LowerASCII(%q) == textlines.LowerASCII(%q), want two keys", pair.first, pair.second)
			}
		})
	}
}

func TestArchiveRelWindows(t *testing.T) {
	runConversions(t, archiveRelWindows, []conversionCase{
		{"a drive path loses its colon", "C:/a", "/C/a"},
		{"a lower-case drive is upper-cased", "c:/a", "/C/a"},
		{"a bare drive becomes its letter", "D:", "/D"},
		{"a UNC path loses one slash", "//s/sh/a", "/s/sh/a"},
		{"the rest below an image root is unchanged", "/Users/a", "/Users/a"},
		{"the empty rest is unchanged", "", ""},
		{"a drive-relative path is unchanged", "C:a", "C:a"},
	})
}

func TestIsAbsWindows(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"C:/x", true}, {`C:\x`, true}, {"C:", true}, {"c:/", true},
		{"//s/sh", true}, {`\\s\sh`, true}, {`\\?\C:\x`, true},
		{`\x`, false}, {"/x", false}, {"x", false}, {"C:x", false},
		{"//", false}, {"///x", false}, {"", false}, {`1:\x`, false},
	}
	for _, testCase := range cases {
		t.Run(fmt.Sprintf("%s is absolute: %t", testCase.path, testCase.want), func(t *testing.T) {
			if got := isAbsWindows(testCase.path); got != testCase.want {
				t.Fatalf("isAbsWindows(%s) = %t", testCase.path, got)
			}
		})
	}
}

func TestSystemBaseWindows(t *testing.T) {
	cases := []struct {
		name        string
		root        string
		systemDrive string
		want        string
	}{
		{"an image root wins", "D:/img", "C:", "D:/img"},
		{"the system drive is the live base", "", "E:", "E:"},
		{"a system drive with a separator is canonicalised", "", `E:\`, "E:"},
		{"no system drive falls back to C:", "", "", "C:"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := systemBaseWindows(testCase.root, testCase.systemDrive); got != testCase.want {
				t.Fatalf("systemBaseWindows() = %s, want %s", got, testCase.want)
			}
		})
	}
}

// customError wraps an error without being a type nativeErrors knows.
type customError struct{ inner error }

// Error puts a prefix before the inner error's text.
func (e customError) Error() string { return "custom: " + e.inner.Error() }

// Unwrap returns the inner error.
func (e customError) Unwrap() error { return e.inner }

func TestNativeErrorsRewritesPaths(t *testing.T) {
	pathError := &fs.PathError{Op: "open", Path: "C:/Users/a", Err: syscall.ENOENT}
	linkError := &os.LinkError{Op: "symlink", Old: "C:/t", New: "//s/sh/l", Err: syscall.EEXIST}
	cases := []struct {
		name  string
		input error
		want  string
	}{
		{"a path error gets the native path", pathError,
			`open C:\Users\a: ` + syscall.ENOENT.Error()},
		{"a link error gets both names native", linkError,
			`symlink C:\t \\s\sh\l: ` + syscall.EEXIST.Error()},
		{"a wrapped path error is rewritten under its wrapper",
			fmt.Errorf("stage C:/Users/a: %w", pathError),
			`stage C:/Users/a: open C:\Users\a: ` + syscall.ENOENT.Error()},
		{"a path error inside a path error is rewritten too",
			&fs.PathError{Op: "copy", Path: "C:/x", Err: pathError},
			`copy C:\x: open C:\Users\a: ` + syscall.ENOENT.Error()},
		{"an unknown wrapper type keeps its own text",
			customError{pathError}, `custom: open C:\Users\a: ` + syscall.ENOENT.Error()},
		{"an extended path loses its prefix",
			&fs.PathError{Op: "lstat", Path: `\\?\C:\a`, Err: syscall.ENOENT},
			`lstat C:\a: ` + syscall.ENOENT.Error()},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, got := nativeErrors(testCase.input, nativeWindows)
			if got.Error() != testCase.want {
				t.Fatalf("nativeErrors() = %s, want %s", got, testCase.want)
			}
		})
	}
}

func TestNativeErrorsKeepsTheChain(t *testing.T) {
	original := &fs.PathError{Op: "open", Path: "C:/a", Err: syscall.ENOENT}
	wrapped := fmt.Errorf("stage: %w", customError{original})
	changed, rewritten := nativeErrors(wrapped, nativeWindows)
	if !changed {
		t.Fatal("a chain holding a path error was not rebuilt")
	}
	if !errors.Is(rewritten, fs.ErrNotExist) {
		t.Fatal("the rebuilt chain no longer matches fs.ErrNotExist")
	}
	var pathError *fs.PathError
	if !errors.As(rewritten, &pathError) || pathError.Path != `C:\a` {
		t.Fatalf("the rebuilt chain holds %v, want the native path", pathError)
	}
	if original.Path != "C:/a" {
		t.Fatalf("the input error was modified: %s", original.Path)
	}
}

func TestNativeErrorsLeavesOtherErrors(t *testing.T) {
	plain := errors.New("C:/a failed")
	wrapped := fmt.Errorf("outer: %w", plain)
	for _, input := range []error{nil, plain, wrapped} {
		t.Run(fmt.Sprintf("%v comes back as it was", input), func(t *testing.T) {
			changed, got := nativeErrors(input, nativeWindows)
			if changed || got != input {
				t.Fatalf("nativeErrors(%v) = %t, %v", input, changed, got)
			}
		})
	}
}

func TestOpenedWindowsIsThePathWin32Opens(t *testing.T) {
	cases := []struct{ name, path, want string }{
		{"a missing name before .. is removed by name", "C:/img/Users/alice/nope/../j/x",
			"C:/img/Users/alice/j/x"},
		{"a link before .. is removed by name too", "C:/img/Users/alice/link/../secret",
			"C:/img/Users/alice/secret"},
		{". and empty segments go", `C:\img\.\a\\b\.`, "C:/img/a/b"},
		{".. never climbs past the drive", "C:/../../x", "C:/x"},
		{"a UNC share keeps its server and share", "//srv/share/a/../b", "//srv/share/b"},
		{"trailing dots and spaces stay, as \\\\?\\ keeps them", "C:/img/j./x ", "C:/img/j./x "},
		{"a relative path is unchanged", "a/../b", "a/../b"},
		{"a drive-relative path is unchanged", "C:a/../b", "C:a/../b"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := openedWindows(testCase.path); got != testCase.want {
				t.Errorf("openedWindows(%q) = %q, want %q", testCase.path, got, testCase.want)
			}
			if extended := extendedWindows(testCase.path); extended != testCase.path &&
				fromWindows(extended) != testCase.want {
				t.Errorf("Extended opens %q, not the checked %q", extended, testCase.want)
			}
		})
	}
}
