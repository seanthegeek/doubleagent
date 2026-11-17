package analyze

import "testing"

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
			if got := posixPathText(test.path); got != test.posix {
				t.Errorf("POSIX form %q, want %q", got, test.posix)
			}
			if got := windowsPathText(test.path); got != test.windows {
				t.Errorf("Windows form %q, want %q", got, test.windows)
			}
		})
	}
}
