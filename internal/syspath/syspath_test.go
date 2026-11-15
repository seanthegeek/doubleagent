package syspath

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"testing"
)

type conversionCase struct {
	name  string
	input string
	want  string
}

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
	runConversions(t, foldASCII, []conversionCase{
		{"ASCII capitals are lowered", "C:/Users/Alice", "c:/users/alice"},
		{"bytes beyond ASCII are kept", "C:/\xc3\x89t\xc3\xa9", "c:/\xc3\x89t\xc3\xa9"},
		{"invalid UTF-8 is kept", "A\xff", "a\xff"},
	})
}

func TestFoldUpper(t *testing.T) {
	runConversions(t, foldUpper, []conversionCase{
		{"ASCII letters are upper-cased", "C:/Users/alice", "C:/USERS/ALICE"},
		{"an accented letter is upper-cased", "\u00e9mile", "\u00c9MILE"},
		{"an upper-case accented letter is kept", "\u00c9mile", "\u00c9MILE"},
		{"sharp s has no simple upper case and is kept", "stra\u00dfe", "STRA\u00dfE"},
		{"dotless i becomes the ASCII I, shortening the text", "\u0131x", "IX"},
		{"dotted capital I is kept", "\u0130x", "\u0130X"},
		{"a lone surrogate's bytes are kept", "a\xed\xa0\x80b", "A\xed\xa0\x80B"},
		{"other invalid UTF-8 is kept", "a\xff\xc3", "A\xff\xc3"},
		{"the replacement character itself is kept", "\ufffda", "\ufffdA"},
		{"the empty text stays", "", ""},
	})
}

func TestFoldUpperMakesNTFSCaseVariantsOneKey(t *testing.T) {
	cases := []struct {
		name          string
		first, second string
		same          bool
	}{
		{"\u00c9 and \u00e9 are one name", "C:/Users/\u00c9mile", "c:/users/\u00e9mile", true},
		{"dotless i and i are one name", "\u0131", "i", true},
		{"dotted capital I and i are two names", "\u0130", "i", false},
		{"sharp s and ss are two names", "\u00df", "ss", false},
		{"two different lone surrogates are two names", "\xed\xa0\x80", "\xed\xa0\x81", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := foldUpper(test.first) == foldUpper(test.second); got != test.same {
				t.Errorf("foldUpper(%q) == foldUpper(%q) is %t, want %t", test.first, test.second,
					got, test.same)
			}
		})
	}
}

func TestMatchFoldedPrefix(t *testing.T) {
	cases := []struct {
		name, path, prefix string
		restIndex          int
		found              bool
	}{
		{"an ASCII prefix in another case matches", "C:/Users/Alice/x", "c:/users/alice/", 15, true},
		{"an accented prefix in another case matches", "C:/Users/\u00c9mile/x", "c:/users/\u00e9mile/",
			len("C:/Users/\u00c9mile/"), true},
		{"a path whose fold is shorter than the prefix's ends the cut in the path",
			"/\u0131/rest", "/i/", len("/\u0131/"), true},
		{"a prefix whose fold is shorter than the path's ends the cut in the path",
			"/i/rest", "/\u0131/", len("/i/"), true},
		{"the empty prefix matches at the start", "C:/a", "", 0, true},
		{"a longer prefix does not match", "C:/a", "C:/a/b", 0, false},
		{"a different letter does not match", "C:/b", "C:/a", 0, false},
		{"a prefix ending inside a character does not match", "/\u00e9", "/\xc3", 0, false},
		{"lone surrogate bytes match only themselves", "/\xed\xa0\x80/x", "/\xed\xa0\x80/",
			len("/\xed\xa0\x80/"), true},
		{"sharp s does not match ss", "/stra\u00dfe", "/STRASS", 0, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			restIndex, found := matchFoldedPrefix(test.path, test.prefix)
			if restIndex != test.restIndex || found != test.found {
				t.Errorf("matchFoldedPrefix(%q, %q) = %d, %t, want %d, %t", test.path, test.prefix,
					restIndex, found, test.restIndex, test.found)
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

func (e customError) Error() string { return "custom: " + e.inner.Error() }
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
