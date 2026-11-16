package collect

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/shortname"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/walk"
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
			if got := absoluteProject(cwd, test.project, nil); got != test.want {
				t.Errorf("absoluteProject(%q) = %q, want %q", test.project, got, test.want)
			}
		})
	}
}

func TestRootAndOutDirTakeTheDisksSpelling(t *testing.T) {
	base := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	if err := os.MkdirAll(syspath.ToNative(base+"/\u00c9mile/Sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, path, want string }{
		{"existing components take the disk's case, an accented letter's too",
			syspath.ToNative(base + "/\u00e9mile/sub"), base + "/\u00c9mile/Sub"},
		{"a missing tail stays as typed", syspath.ToNative(base + "/\u00e9MILE/sub/New"),
			base + "/\u00c9mile/Sub/New"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got, _ := absoluteRoot("", test.path); got != test.want {
				t.Errorf("absoluteRoot(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestProjectsAreRespelledOnlyBelowAnImageRoot(t *testing.T) {
	base := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	for _, dir := range []string{"img/Proj", "other"} {
		if err := os.MkdirAll(syspath.ToNative(base+"/"+dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name, root string
		projects   []string
		want       []string
	}{
		{"a path outside the image root stays as typed, never looked at", base + "/img",
			[]string{base + "/OTHER"}, []string{base + "/OTHER"}},
		{"a path below the image root takes the disk's spelling", base + "/img",
			[]string{base + "/img/proj"}, []string{base + "/img/Proj"}},
		{"a live run respells from the drive and lists a directory once", "",
			[]string{base + "/OTHER", base + "/other"}, []string{base + "/other"}},
		{"a path's . and .. are removed by name, as Win32 opens it, and it is listed once",
			base + "/img", []string{base + "/img/../img/./proj", base + "/img/Proj"},
			[]string{base + "/img/Proj"}},
		{"a path whose .. leaves the image root by name is outside it", base + "/img",
			[]string{base + "/img/../other"}, []string{base + "/other"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := respellProjects(test.projects, test.root, test.root); !reflect.DeepEqual(got, test.want) {
				t.Errorf("respellProjects(%q, %q) = %q, want %q", test.projects, test.root, got, test.want)
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

func TestAProjectTypedLikeTheRootStaysUnderIt(t *testing.T) {
	// The working directory is typed with short names, which the root
	// would lose.
	work, _ := shortname.Dir(t)
	if err := os.MkdirAll(work+`\img\proj`, 0o755); err != nil {
		t.Fatal(err)
	}
	getwd := func() (string, error) { return syspath.FromNative(work), nil }
	options, err := resolvePaths(Options{Root: "img", Projects: []string{`img\proj`}}, getwd)
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Projects) != 1 || !syspath.HasPrefix(options.Projects[0], options.Root+"/") {
		t.Fatalf("projects %q, want one under the root %q", options.Projects, options.Root)
	}
	if name, out := walk.NewRootCheck(options.Root).OutOfRoot(options.Projects[0]); out {
		t.Errorf("the project %q counts as out of the root %q at %q", options.Projects[0],
			options.Root, name)
	}
}
