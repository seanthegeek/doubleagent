package projects

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"reflect"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/follow"
	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"github.com/seanthegeek/doubleagent/internal/users"
)

func TestDiscoverReadsDrivePathsAndIgnoresCase(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	writeTree(t, root, map[string]string{
		"home/alice/.claude/history.jsonl": `{"project":"C:\\proj"}` + "\n" +
			`{"project":"/PROJ"}` + "\n" + `{"project":"/HOME/ALICE"}` + "\n" +
			`{"project":"c:/home/alice/inside/"}` + "\n",
		"home/alice/.codex/sessions/s.jsonl": `{"cwd":"/HOME/ALICE/documents/codex/sub"}` + "\n" +
			`{"cwd":"/home/alice/.CODEX/worktrees/w"}` + "\n",
	}, "proj", "home/alice/inside", "home/alice/other", "home/alice/Documents/Codex/sub",
		"home/alice/.codex/worktrees/w")
	homes := []users.Home{{User: "alice", Path: root + "/home/alice"}}
	extra := []string{root + "/HOME/alice/INSIDE", root + "/HOME/ALICE/other"}
	got, _, err := Discover(context.Background(), homes, root, extra, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The drive paths land under the image root; a case variant of a found
	// path or of the home is the same directory; the Codex filters fold.
	want := []Project{{"alice", root + "/proj"}, {"alice", root + "/home/alice/inside"},
		{"alice", root + "/HOME/ALICE/other"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverRespellsAProjectAsTheDiskHoldsIt(t *testing.T) {
	root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	writeTree(t, root, map[string]string{
		"Users/alice/.claude/history.jsonl": "{\"project\":\"C:\\\\\u00c9mile\"}\n" +
			`{"project":"C:\\IMG"}` + "\n",
	}, "\u00e9mile", "img", "\u0131mg")
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The accented name takes the disk's case; IMG names img, never the
	// dotless-i img NTFS keeps beside it.
	want := []Project{{"alice", root + "/\u00e9mile"}, {"alice", root + "/img"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverReadsDriveKeysAndStrings(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		content string
		project string
	}{
		{"a Gemini projects.json drive key", ".gemini/projects.json", `{"projects":{"C:\\gem":"gem"}}`, "gem"},
		{"a pi trust.json drive key", ".pi/agent/trust.json", `{"C:\\pitrust":true}`, "pitrust"},
		{"a muse trust.json drive key", ".config/muse/trust.json",
			`{"schema_version":1,"projects":{"C:\\musetrust":{"decision":"trusted"}}}`, "musetrust"},
		{"a relocated Claude Code home's .claude.json drive key", ".claude-work/.claude.json",
			`{"projects":{"C:\\ccwork":{"allowedTools":[]}}}`, "ccwork"},
		{"the directory a Twinny manifest's drive paths share", ".twinny/embeddings/twinproj/manifest.json",
			`{"files":{"C:\\twinproj\\src\\a.ts":{"hash":"h1"},"C:\\twinproj\\b.ts":{"hash":"h2"}}}`,
			"twinproj"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
			writeTree(t, root, map[string]string{"Users/alice/" + testCase.source: testCase.content},
				testCase.project)
			homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
			got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := []Project{{"alice", root + "/" + testCase.project}}; !reflect.DeepEqual(got, want) {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

func TestProjectComparisonsIgnoreCaseWhereTheFileSystemDoes(t *testing.T) {
	one := func(string, string) bool { return true }
	two := func(string, string) bool { return false }
	t.Run("an owner's home matches in any case", func(t *testing.T) {
		if got := projectOwner([]users.Home{{User: "alice", Path: "C:/Users/Alice"}}, "c:/users/alice/p"); got != "alice" {
			t.Errorf("projectOwner = %q, want alice", got)
		}
	})
	t.Run("a home spelled as the path owns it before one matching in another case",
		func(t *testing.T) {
			list := []users.Home{{User: "Alice", Path: "C:/Users/Alice"},
				{User: "alice", Path: "C:/Users/alice"}}
			if got := projectOwner(list, "C:/Users/alice/p"); got != "alice" {
				t.Errorf("projectOwner = %q, want alice", got)
			}
		})
	t.Run("two spellings of one directory merge into the first", func(t *testing.T) {
		got := mergeProjectOwners([]Project{{"", "C:/P"}, {"bob", "c:/p"}}, one)
		if want := []Project{{"bob", "C:/P"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("two directories differing only in case stay two", func(t *testing.T) {
		got := mergeProjectOwners([]Project{{"", "C:/P"}, {"bob", "c:/p"}}, two)
		if want := []Project{{"", "C:/P"}, {"bob", "c:/p"}}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("a home in another case is not a project", func(t *testing.T) {
		root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
		writeTree(t, root, nil, "Home")
		got := keepProjectDirs([]Project{{"u", root + "/HOME"}}, []users.Home{{User: "u", Path: root + "/home"}}, nil,
			follow.SameEntry(nil))
		if got != nil {
			t.Errorf("got %q, want none", got)
		}
	})
	t.Run("a directory differing from a home only in case is a project", func(t *testing.T) {
		root := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
		writeTree(t, root, nil, "Home")
		projects := []Project{{"u", root + "/Home"}}
		if got := keepProjectDirs(projects, []users.Home{{User: "u", Path: root + "/home"}}, nil, two); !reflect.DeepEqual(got,
			projects) {
			t.Errorf("got %q, want %q", got, projects)
		}
	})
}

func TestLocalProjectPathRespellsBelowTheRootUpToALink(t *testing.T) {
	base := fsmeta.OnDiskPath("", syspath.FromNative(t.TempDir()))
	writeTree(t, base, nil, "Img/Proj/Sub", "Img/other/Real/Inner")
	makeJunction(t, base+"/Img/other/Real", base+"/Img/Link")
	root := base + "/img" // the root as typed, which is never respelled
	cases := []struct{ name, value, want string }{
		{"components below the root take the disk's spelling", `C:\proj\sub`, root + "/Proj/Sub"},
		{"a junction is respelled and nothing behind it is", `C:\link\inner`, root + "/Link/inner"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, ok := localProjectPath(test.value, root, platformRules())
			if !ok || got != test.want {
				t.Errorf("localProjectPath(%q) = %q, %t, want %q", test.value, got, ok, test.want)
			}
		})
	}
}

// utf16Pointer returns text as a NUL-terminated UTF-16 string.
func utf16Pointer(t *testing.T, text string) *uint16 {
	t.Helper()
	pointer, err := windows.UTF16PtrFromString(text)
	if err != nil {
		t.Fatal(err)
	}
	return pointer
}

// makeJunction makes link a junction to the directory target, both
// canonical, by setting on a new empty directory the mount-point reparse
// data mklink /J writes. target need not exist.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(syspath.ToNative(link), 0o755); err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(utf16Pointer(t, syspath.ToNative(link)), windows.GENERIC_WRITE,
		0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	native := syspath.ToNative(target)
	data := mountPointData(`\??\`+native+`\`, native)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// mountPointData lays out a REPARSE_DATA_BUFFER holding a mount-point
// (junction) reparse buffer: the tag, the data length, a reserved word, the
// offsets and lengths in bytes of the two names, then the substitute name
// the system follows and the print name shown to people, each ending in NUL.
func mountPointData(substitute, printName string) []byte {
	substituteText := utf16.Encode([]rune(substitute))
	printText := utf16.Encode([]rune(printName))
	names := make([]uint16, 0, len(substituteText)+len(printText)+2)
	names = append(append(names, substituteText...), 0)
	names = append(append(names, printText...), 0)
	substituteBytes := uint16(2 * len(substituteText))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_MOUNT_POINT)
	data = binary.LittleEndian.AppendUint16(data, uint16(8+2*len(names)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes+2)
	data = binary.LittleEndian.AppendUint16(data, uint16(2*len(printText)))
	for _, unit := range names {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

// linkDirectory makes link a junction to the directory target, both
// canonical: the runner may not create symlinks, and the out-of-root test
// treats a junction as a symlinked directory.
func linkDirectory(t *testing.T, target, link string) {
	t.Helper()
	makeJunction(t, target, link)
}

// linkRelative makes link a symlink holding the relative target as written,
// skipping the test where the account may not create symlinks: a junction
// cannot hold a relative target.
func linkRelative(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(syspath.ToNative(target), syspath.ToNative(link))
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skipf("creating a symbolic link needs the symbolic-link privilege: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
}
