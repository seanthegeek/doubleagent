//go:build unix

package projects

import (
	"context"
	"errors"
	"os"
	"reflect"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/users"
	"github.com/seanthegeek/doubleagent/internal/walk"
)

// holdCaseVariants makes each pair of directories under root, the two names
// differing only in the case of their letters, and skips the test where the
// disk ignores case and so holds one entry for both.
func holdCaseVariants(t *testing.T, root string, pairs ...[2]string) {
	t.Helper()
	for _, pair := range pairs {
		writeTree(t, root, nil, pair[0])
		if err := os.Mkdir(root+"/"+pair[1], 0o755); errors.Is(err, os.ErrExist) {
			t.Skipf("the disk under %s ignores case, so %q and %q are one entry", root, pair[0], pair[1])
		} else if err != nil {
			t.Fatal(err)
		}
	}
}

// projectsAt returns alice's projects at each path below root.
func projectsAt(root string, paths ...string) []Project {
	var projects []Project
	for _, path := range paths {
		projects = append(projects, Project{"alice", root + "/" + path})
	}
	return projects
}

func TestDiscoverTakesTheExactSpellingOfADriveValueOverACaseVariant(t *testing.T) {
	root := imageRoot(t)
	holdCaseVariants(t, root, [2]string{"Proj", "proj"}, [2]string{"jos\u00e9", "JOS\u00c9"})
	writeTree(t, root, nil, "Proj/sub", "proj/sub", "jos\u00e9/sub", "JOS\u00c9/sub")
	got, _ := discoverHistory(t, root, nil, `C:\proj\SUB`, `C:\Proj\SUB`, `C:\JOS`+"\u00c9"+`\SUB`,
		`C:\jos`+"\u00e9"+`\SUB`)
	// Each part below the exact one is spelled too, so the exact entry was
	// the one taken.
	want := projectsAt(root, "proj/sub", "Proj/sub", "JOS\u00c9/sub", "jos\u00e9/sub")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverDropsADriveValueSeveralEntriesMatchOnlyIgnoringCase(t *testing.T) {
	root := imageRoot(t)
	holdCaseVariants(t, root, [2]string{"Proj", "proj"}, [2]string{"jos\u00e9", "JOS\u00c9"})
	writeTree(t, root, nil, "kept")
	got, _ := discoverHistory(t, root, nil, `C:\PROJ`, `C:\Jos`+"\u00c9", `C:\KEPT`)
	if want := projectsAt(root, "kept"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want only %q: an ambiguous part names no entry", got, want)
	}
}

func TestLocalProjectPathSpellsADriveValueUpToALink(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, nil, "Img/Proj/Sub", "Img/Real/Inner")
	linkDirectory(t, root+"/Img/Real", root+"/Img/Link")
	cases := []struct{ name, value, want string }{
		{"each part takes the disk's spelling", `C:\img\proj\sub`, root + "/Img/Proj/Sub"},
		{"a link is spelled and nothing behind it is", `C:\img\link\inner`, root + "/Img/Link/inner"},
		{"a missing part and the rest stay as written", `C:\img\Gone\proj`, root + "/Img/Gone/proj"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, ok := localProjectPath(test.value, root, platformRules(root, nil, nil))
			if !ok || got != test.want {
				t.Errorf("localProjectPath(%q) = %q, %t, want %q", test.value, got, ok, test.want)
			}
		})
	}
}

func TestDiscoverListsNothingBehindALinkOutOfTheImage(t *testing.T) {
	root, outside := imageRoot(t), imageRoot(t)
	writeTree(t, outside, nil, "Proj")
	linkDirectory(t, outside, root+"/Out")
	got, _ := discoverHistory(t, root, walk.NewRootCheck(root).Exit, `C:\out\proj`)
	// The link is spelled from its own directory; proj, behind it, is kept as
	// written, untested, as a POSIX value leading out of the root is.
	if want := projectsAt(root, "Out/proj"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverLooksUpAGlobCharacterAsALiteralName(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, nil, "Proj[1]/A*b", "Proj[1]/Axxb", "Proj1/A*b", "Q?r", "Qxr")
	got, _ := discoverHistory(t, root, nil, `C:\proj[1]\a*b`, `C:\q?R`)
	if want := projectsAt(root, "Proj[1]/A*b", "Q?r"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverLooksAPartUpAsWrittenWhereADirectoryCannotBeListed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser lists a directory whatever its mode")
	}
	root := imageRoot(t)
	writeTree(t, root, nil, "Locked/Proj/sub")
	if err := os.Chmod(root+"/Locked", 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root+"/Locked", 0o755) })
	got, _ := discoverHistory(t, root, nil, `C:\locked\Proj\SUB`)
	// Proj, found by lstat, is listed in turn, so SUB takes the disk's spelling.
	if want := projectsAt(root, "Locked/Proj/sub"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverMatchesANameThatIsNotUTF8ByItsBytesOnly(t *testing.T) {
	root := imageRoot(t)
	for _, name := range []string{"A\xfeb", "x\xffy"} {
		if err := os.Mkdir(root+"/"+name, 0o755); errors.Is(err, syscall.EILSEQ) {
			t.Skipf("the disk under %s takes only UTF-8 names: %v", root, err)
		} else if err != nil {
			t.Fatal(err)
		}
	}
	got, _ := discoverHistory(t, root, nil, `C:\a`+"\xffb", `C:\x`+"\ufffdy", `C:\A`+"\xfeb")
	if want := projectsAt(root, "A\xfeb"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want only %q", got, want)
	}
}

func TestFoldedNameComparesValidUTF8Only(t *testing.T) {
	for _, test := range []struct {
		name, part, entry string
		want              bool
	}{
		{"letters of any case are equal", "JOS\u00c9", "jos\u00e9", true},
		{"two bytes that are not UTF-8 are not equal", "a\xffb", "A\xfeb", false},
		{"a replacement character is not a byte that is not UTF-8", "x\ufffdy", "x\xffy", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := foldedName(test.part, test.entry); got != test.want {
				t.Errorf("foldedName(%q, %q) = %t, want %t", test.part, test.entry, got, test.want)
			}
		})
	}
}

func TestSpellListsEachDirectoryOnce(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, nil, "Proj")
	speller := newDiskSpeller(nil, nil)
	first := speller.spell(root, root+"/proj")
	if err := os.Rename(root+"/Proj", root+"/Renamed"); err != nil {
		t.Fatal(err)
	}
	// The second call reads the listing the first one kept.
	if second := speller.spell(root, root+"/proj"); first != root+"/Proj" || second != first {
		t.Errorf("spelled %q, then %q; want %q twice", first, second, root+"/Proj")
	}
}

func TestDiscoverKeepsThePOSIXRuleForEveryOtherValueInAnImage(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, map[string]string{"Users/alice/.claude/history.jsonl": `{"project":"/srv/a\b"}` +
		"\n" + `{"project":"/srv\/x"}` + "\n" + `{"project":"file:///srv/x"}` + "\n" +
		`{"project":" /srv/x"}` + "\n" + `{"project":"srv\\x"}` + "\n"}, `srv/a\b`, "srv/x")
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The value's bytes are kept, a backslash in a name included, and only
	// an escaped slash is undone; no URI is decoded and no blank trimmed.
	if want := projectsAt(root, `srv/a\b`, "srv/x"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiscoverDropsADriveValueOnALiveRun(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, map[string]string{
		"Users/alice/.claude/history.jsonl": `{"project":"C:\\proj"}` + "\n" +
			`{"project":"` + root + `/srv"}` + "\n",
	}, "proj", "srv")
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, _, err := Discover(context.Background(), homes, "", nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := projectsAt(root, "srv"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q: a drive path names no place on a live Linux or macOS host", got, want)
	}
	if rules := platformRules("", nil, nil); !reflect.DeepEqual(rules, pathRules{}) {
		t.Errorf("live rules %+v, want the POSIX rules", rules)
	}
}

func TestDiscoverSpellsADriveURLAsTheDiskHoldsIt(t *testing.T) {
	root := imageRoot(t)
	writeTree(t, root, map[string]string{"Users/alice/.gemini/config/projects/t.json": "{\n" +
		`  "folderUri":  "file:///c%3A/users/ALICE/Demo"` + "\n}\n"}, "Users/alice/demo")
	homes := []users.Home{{User: "alice", Path: root + "/Users/alice"}}
	got, _, err := Discover(context.Background(), homes, root, nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := projectsAt(root, "Users/alice/demo"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
