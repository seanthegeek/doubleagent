package input

import "testing"

// Expected values from str(), .name and .parent of PurePosixPath and
// PureWindowsPath under Python 3.14.
func TestPathFormPrintsAsPathlibDoes(t *testing.T) {
	cases := []struct {
		name, path      string
		windows         bool
		text, base, top string
	}{
		{"an empty path is a dot", "", false, ".", "", "."},
		{"a dot alone", ".", false, ".", "", "."},
		{"a leading dot component goes", "./a", false, "a", "a", "."},
		{"an inner dot component goes", "a/./b", false, "a/b", "b", "a"},
		{"a doubled slash collapses", "a//b", false, "a/b", "b", "a"},
		{"two leading slashes stay", "//a", false, "//a", "a", "//"},
		{"three leading slashes become one", "///a", false, "/a", "a", "/"},
		{"a trailing slash goes", "a/", false, "a", "a", "."},
		{"the root alone", "/", false, "/", "", "/"},
		{"two slashes alone", "//", false, "//", "", "//"},
		{"dot-dot is kept", "a/../b", false, "a/../b", "b", "a/.."},
		{"dot-dot alone", "..", false, "..", "..", "."},
		{"a trailing dot goes", "a/.", false, "a", "a", "."},
		{"a backslash is a name character on POSIX", `a\b`, false, `a\b`, `a\b`, "."},
		{"an absolute path with a trailing slash", "/tmp/x/", false, "/tmp/x", "x", "/tmp"},
		{"slashes become backslashes", "C:/x/y", true, `C:\x\y`, "y", `C:\x`},
		{"a drive path loses its trailing backslash", `C:\x\`, true, `C:\x`, "x", `C:\`},
		{"a drive alone", "C:", true, "C:", "", "C:"},
		{"a drive-relative path", "C:x", true, "C:x", "x", "C:"},
		{"a share path", `\\srv\share\a`, true, `\\srv\share\a`, "a", `\\srv\share\`},
		{"a share gets its root", `\\srv\share`, true, `\\srv\share\`, "", `\\srv\share\`},
		{"an extended path", `\\?\C:\a`, true, `\\?\C:\a`, "a", `\\?\C:\`},
		{"an extended share path", `\\?\UNC\srv\sh\a`, true, `\\?\UNC\srv\sh\a`, "a", `\\?\UNC\srv\sh\`},
		{"a relative Windows path", "a/b", true, `a\b`, "b", "a"},
		{"a rooted path without a drive", "/a", true, `\a`, "a", `\`},
		{"two leading slashes make a share", "//a/b/c", true, `\\a\b\c`, "c", `\\a\b\`},
		{"a leading dot goes on Windows", `.\a`, true, "a", "a", "."},
		{"an inner dot goes on Windows", `a\.\b`, true, `a\b`, "b", "a"},
		{"an empty Windows path is a dot", "", true, ".", "", "."},
		{"a lower-case drive is kept", "c:/Users/bob/", true, `c:\Users\bob`, "bob", `c:\Users`},
		{"three leading slashes and a dot are a drive without a root", "///.", true, `\\\.`, "", `\\\.`},
		{"two slashes alone are a drive", "//", true, `\\`, "", `\\`},
		{"three leading slashes and a name take a root", "///a/b", true, `\\\a\b`, "b", `\\\a\`},
		{"an extended share gets its root", `\\?\UNC\srv\sh`, true, `\\?\UNC\srv\sh\`, "",
			`\\?\UNC\srv\sh\`},
		{"a device path has no root", `\\.\pipe`, true, `\\.\pipe`, "", `\\.\pipe`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			form := parsePathAs(test.path, test.windows)
			got := []string{form.String(), form.name(), form.parent().String()}
			want := []string{test.text, test.base, test.top}
			if got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
				t.Fatalf("%q gives %q, want %q", test.path, got, want)
			}
		})
	}
}

func TestPathFormJoinsAsJoinpathDoes(t *testing.T) {
	cases := []struct {
		name, base, element string
		windows             bool
		want                string
	}{
		{"an absolute element replaces the path", "a", "/b", false, "/b"},
		{"dot-dot in an element is kept", "/a", "b/../c", false, "/a/b/../c"},
		{"a dot base disappears", ".", ".claude", false, ".claude"},
		{"an empty base disappears", "", "x", false, "x"},
		{"an empty element adds nothing", "a", "", false, "a"},
		{"a rooted element keeps the drive", `C:\a`, `\b`, true, `C:\b`},
		{"an element with a drive replaces the path", `C:\a`, "D:b", true, "D:b"},
		{"a relative element is appended", `C:\a`, "b/c", true, `C:\a\b\c`},
		{"a drive alone takes a relative element", "C:", "x", true, "C:x"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := parsePathAs(test.base, test.windows).join(test.element).String()
			if got != test.want {
				t.Fatalf("%q / %q = %q, want %q", test.base, test.element, got, test.want)
			}
		})
	}
}

// The expected texts are Python's str(PurePosixPath(path)) and
// str(PureWindowsPath(path)).
func TestPathTextPrintsAPathAsPathlibDoes(t *testing.T) {
	tests := []struct {
		name, path, posix, windows string
	}{
		{"an empty path is the current directory", "", ".", "."},
		{"a lone dot is the current directory", ".", ".", "."},
		{"repeated separators, dot components and a trailing slash go", "a//b/./c/", "a/b/c",
			`a\b\c`},
		{"two leading slashes are kept", "//srv/x", "//srv/x", `\\srv\x\`},
		{"three leading slashes become one", "///srv/x", "/srv/x", `\\\srv\x`},
		{"a parent component is kept", "/a/../b", "/a/../b", `\a\..\b`},
		{"a leading dot component goes", "./a", "a", "a"},
		{"the root alone stays", "/", "/", `\`},
		{"a drive with slashes takes backslashes", "C:/a//b/./c/", "C:/a/b/c", `C:\a\b\c`},
		{"a drive alone stays", "c:", "c:", "c:"},
		{"a share keeps its root", `\\server\share`, `\\server\share`, `\\server\share\`},
		{"an extended-length path keeps its prefix", `\\?\C:\x`, `\\?\C:\x`, `\\?\C:\x`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parsePosixPath(test.path).String(); got != test.posix {
				t.Errorf("POSIX form %q, want %q", got, test.posix)
			}
			if got := parseWindowsPath(test.path).String(); got != test.windows {
				t.Errorf("Windows form %q, want %q", got, test.windows)
			}
			if got := PathText(test.path); got != parsePath(test.path).String() {
				t.Errorf("PathText = %q, want this platform's form %q", got, parsePath(test.path).String())
			}
		})
	}
}
