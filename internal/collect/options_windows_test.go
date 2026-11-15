package collect

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
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

func TestAbsoluteProjectTakesTheDisksSpelling(t *testing.T) {
	base := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	if err := os.MkdirAll(syspath.ToNative(base+"/\u00c9mile/Sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, project, want string }{
		{"existing components take the disk's case, an accented letter's too",
			syspath.ToNative(base + "/\u00e9mile/sub"), base + "/\u00c9mile/Sub"},
		{"a missing tail stays as typed", syspath.ToNative(base + "/\u00e9MILE/sub/New"),
			base + "/\u00c9mile/Sub/New"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := absoluteProject("", test.project); got != test.want {
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

// platformRootCases are the roots Windows resolves its own way: every root
// is an image, a rooted path is on the current directory's drive and a bare
// drive is its root.
func platformRootCases(base string) []rootCase {
	drive := base[:2]
	return []rootCase{
		{"/ is the current drive's root, an image", "/", drive, ""},
		{"a bare drive is its root, an image", drive, drive, ""},
		{"a drive root is an image too", drive + `\`, drive, ""},
		{"a root in backslashes with dot-dot is resolved by name", `img\etc\..\`, base + "/img", ""},
	}
}
