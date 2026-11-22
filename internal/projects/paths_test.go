package projects

import (
	"slices"
	"testing"
)

// pathCase is one value localProjectPath reads, the image root, the rules and
// the path and acceptance it should give.
type pathCase struct {
	name, value, root string
	rules             pathRules
	want              string
	ok                bool
}

// windowsDrivePathCases returns the cases of TestLocalProjectPath for the
// Windows rule set's drive paths and drive URIs.
func windowsDrivePathCases() []pathCase {
	windows := pathRules{driveLetters: true, decodeEscapes: true, dropPOSIXLive: true}
	return []pathCase{
		{"a drive URI loses its scheme and slash", "file:///C:/x", "", windows, "C:/x", true},
		{"an escaped drive colon is decoded", "file:///C%3A/x", "", windows, "C:/x", true},
		{"a URI is decoded whole and its letter raised", "file:///c%3a/My%20Dir/%C3%A9", "",
			windows, "C:/My Dir/\xc3\xa9", true},
		{"a malformed escape is kept", "file:///C:/a%zz%4", "", windows, "C:/a%zz%4", true},
		{"a JSON-escaped drive path is undone", `C:\\proj\\sub`, "", windows, "C:/proj/sub", true},
		{"a small drive letter is raised", "c:/x", "", windows, "C:/x", true},
		{"blanks and trailing separators are trimmed", " C:\\x\\/ \r", "", windows, "C:/x", true},
		{"a drive root is the bare drive", `C:\`, "", windows, "C:", true},
		{"a drive path in an image is under the root", `C:\Users\a`, "D:/img", windows,
			"D:/img/Users/a", true},
	}
}

// windowsOtherPathCases returns the cases of TestLocalProjectPath for the
// Windows rule set's POSIX paths, cut drive URIs and refused values.
func windowsOtherPathCases() []pathCase {
	windows := pathRules{driveLetters: true, decodeEscapes: true, dropPOSIXLive: true}
	return []pathCase{
		{"a POSIX path is dropped live", "/srv/p", "", windows, "", false},
		{"a POSIX path in an image is under the root", "/srv/p", "D:/img", windows,
			"D:/img/srv/p", true},
		{"an escaped POSIX path is undone before the test", `\/srv\/p`, "D:/img", windows,
			"D:/img/srv/p", true},
		{"a POSIX URI is dropped live", "file:///opt/u", "", windows, "", false},
		{"a POSIX URI in an image is under the root", "file:///opt/u", "D:/img", windows,
			"D:/img/opt/u", true},
		{"a cut drive URI is a drive path live", "/C:/tabbyproj", "", windows, "C:/tabbyproj", true},
		{"a cut drive URI in an image is under the root", "/C:/tabbyproj", "D:/img", windows,
			"D:/img/tabbyproj", true},
		{"a cut escaped drive URI is a drive path live", "/c%3A/Users/x", "", windows,
			"C:/Users/x", true},
		{"a cut escaped drive URI in an image is under the root", "/c%3A/Users/x", "D:/img",
			windows, "D:/img/Users/x", true},
		{"a cut drive URI with a small letter is raised", "/c:/x", "", windows, "C:/x", true},
		{"a cut escaped drive URI with small hex is a drive path", "/C%3a/x", "", windows,
			"C:/x", true},
		{"a cut escaped drive URI with a non-ASCII letter stays POSIX", "/\xc3\xa9%3A/x", "D:/img",
			windows, "D:/img/\xc3\xa9%3A/x", true},
		{"a cut escaped drive URI with nothing after stays POSIX", "/C%3A", "D:/img", windows,
			"D:/img/C%3A", true},
		{"a cut drive URI with nothing after stays POSIX", "/C:", "", windows, "", false},
		{"the image root itself is a path", "/", "D:/img", windows, "D:/img", true},
		{"a bare drive is not a path", "X:", "", windows, "", false},
		{"a relative path is not a path", "proj", "", windows, "", false},
		{"a network path is not a path", `\\\\srv\\share`, "", windows, "", false},
		{"blanks alone are not a path", " \t", "", windows, "", false},
	}
}

// posixPathCases returns the cases of TestLocalProjectPath for the POSIX
// rule set.
func posixPathCases() []pathCase {
	posix := pathRules{}
	return []pathCase{
		{"a cut drive URI is not taken by the POSIX rules", "/C:/x", "/img", posix, "/img/C:/x", true},
		{"the POSIX rules undo an escaped slash and add the root", `/a\/b`, "/img", posix, "/img/a/b", true},
		{"the POSIX rules drop a value starting with an escaped slash", `\/a`, "", posix, "", false},
		{"the POSIX rules leave %20 to the readers", "/a%20b", "", posix, "/a%20b", true},
		{"the POSIX rules keep a trailing slash", "/a/", "", posix, "/a/", true},
		{"the POSIX rules trim no blank", " /a", "", posix, "", false},
		{"the POSIX rules take no drive path", `C:\x`, "", posix, "", false},
		{"the POSIX rules take no URI", "file:///x", "", posix, "", false},
	}
}

func TestLocalProjectPath(t *testing.T) {
	for _, test := range slices.Concat(windowsDrivePathCases(), windowsOtherPathCases(), posixPathCases()) {
		t.Run(test.name, func(t *testing.T) {
			got, ok := localProjectPath(test.value, test.root, test.rules)
			if got != test.want || ok != test.ok {
				t.Errorf("localProjectPath(%q, %q) = %q, %v; want %q, %v",
					test.value, test.root, got, ok, test.want, test.ok)
			}
		})
	}
}
