package collect

import (
	"errors"
	"reflect"
	"testing"
)

func TestAbsoluteProjectOnWindows(t *testing.T) {
	cwd := `C:\work\here`
	for _, test := range []struct {
		name, project, want string
	}{
		{"an absolute path is made canonical", `D:\a\b`, "D:/a/b"},
		{"an absolute path in forward slashes is kept", "D:/a/b", "D:/a/b"},
		{"dot segments resolve lexically", `D:\a\..\b\.\c`, "D:/b/c"},
		{"trailing separators are trimmed", `D:\a\\`, "D:/a"},
		{"a bare drive is its root", "X:", "X:"},
		{"a drive root stays the drive", `X:\`, "X:"},
		{"a relative path joins the current directory", `sub\..\x`, "C:/work/here/x"},
		{"a missing relative path joins it too", "gone", "C:/work/here/gone"},
		{"a rooted path is on the current drive", `\top`, "C:/top"},
		{"a rooted path in forward slashes is too", "/top", "C:/top"},
		{"a drive-relative path on the current drive joins the directory", "c:sub",
			"C:/work/here/sub"},
		{"a drive-relative path on another drive starts at its root", "E:sub", "E:/sub"},
		{"a network path keeps its share", `\\srv\share\p\`, "//srv/share/p"},
		{"dot-dot never climbs above a share", `\\srv\share\..\p`, "//srv/share/p"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := absoluteProject(cwd, test.project); got != test.want {
				t.Errorf("absoluteProject(%q) = %q, want %q", test.project, got, test.want)
			}
		})
	}
}

func TestResolveProjectsOnWindows(t *testing.T) {
	getwd := func() (string, error) { return `C:\work`, nil }
	failing := func() (string, error) { return "", errors.New("no cwd") }
	cases := []struct {
		name  string
		input []string
		getwd func() (string, error)
		want  []string
	}{
		{"case variants are listed once in the first spelling",
			[]string{`C:\Work\P`, `c:\work\p\`, "p"}, getwd, []string{"C:/Work/P"}},
		{"an empty path is dropped", []string{"", `D:\a`}, getwd, []string{"D:/a"}},
		{"an absolute path needs no current directory", []string{`D:\a`}, failing,
			[]string{"D:/a"}},
		{"no path gives none", nil, getwd, nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ResolveProjects(Options{Projects: testCase.input}, testCase.getwd)
			if err != nil || !reflect.DeepEqual(got.Projects, testCase.want) {
				t.Errorf("projects %q, error %v; want %q", got.Projects, err, testCase.want)
			}
		})
	}
}
