package walk

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

func TestVolumeOf(t *testing.T) {
	cases := map[string]string{
		"C:/Users/alice":      "C:",
		"d:":                  "d:",
		"//server/share/a/b":  "//server/share",
		"//server/share":      "//server/share",
		"//server":            "",
		"//server/":           "",
		"/no/drive":           "",
		"1:/not/a/drive":      "",
		"relative/C:/x":       "",
		"":                    "",
		"///server/share/a/b": "",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			if got := volumeOf(path); got != want {
				t.Errorf("volumeOf(%q) = %q, want %q", path, got, want)
			}
		})
	}
}

// rootedLinkProbes are the image's probes, except that link is a symlink
// whose target is target, as CreateSymbolicLink would store it.
func rootedLinkProbes(link, target string) probes {
	return probes{
		isLink: func(path string) (bool, error) {
			if syspath.Equal(path, link) {
				return true, nil
			}
			return isLinkProbe(path)
		},
		reparse: func(path string) (fsmeta.Reparse, error) {
			if syspath.Equal(path, link) {
				return storedSymlink(target), nil
			}
			return fsmeta.ReadReparse(path)
		},
		respell: respellFromDrive,
	}
}

// storedSymlink is the reparse data CreateSymbolicLink stores for target: a
// drive path as `\??\X:\...` without the relative flag, anything else as
// given with it.
func storedSymlink(target string) fsmeta.Reparse {
	if len(target) >= 2 && target[1] == ':' {
		return fsmeta.Reparse{Tag: fsmeta.TagSymlink, Substitute: `\??\` + target}
	}
	return fsmeta.Reparse{Tag: fsmeta.TagSymlink, Substitute: target, Relative: true}
}

func TestRootedLinkTargetResolvesOnTheLinksDrive(t *testing.T) {
	root := physicalTemp(t)
	writeFiles(t, root, "srv/real/proj/AGENTS.md")
	mustDo(t, os.MkdirAll(root+"/home/u/ext", 0o755))
	link := root + "/home/u/ext"
	// The root without its drive, in native form: a target with one leading
	// separator that names the root's own srv/real.
	rootedInside := syspath.ToNative(root[2:] + "/srv/real")
	cases := []struct {
		name   string
		target string
		want   rootScope
	}{
		{"a rooted target outside the root leads out of it", `\outside-of-every-image`,
			rootScope{name: link}},
		{"a rooted target naming a place inside the root is walked", rootedInside, rootScope{}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := projectScope(root, link+"/proj", rootedLinkProbes(link, testCase.target))
			if got != testCase.want {
				t.Errorf("scope %+v, want %+v", got, testCase.want)
			}
		})
	}
}

func TestRootedSymlinkLeavesTheRoot(t *testing.T) {
	root := physicalTemp(t)
	mustDo(t, os.MkdirAll(root+"/home/u", 0o755))
	link := root + "/home/u/gone"
	err := os.Symlink(`\no-such-host-dir-for-this-test`, syspath.ToNative(link))
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skipf("creating a symbolic link needs a privilege this account lacks: %v", err)
	}
	mustDo(t, err)
	target, err := os.Readlink(syspath.ToNative(link))
	mustDo(t, err)
	if !strings.HasPrefix(target, `\no-such`) {
		t.Fatalf("readlink = %q, want the rooted target as written", target)
	}
	if !ProjectLeavesRoot(root, link+"/proj") {
		t.Error("a project under a link to a rooted target outside the root does not leave it")
	}
	if name, out := ProjectOutOfRoot(root, link+"/proj"); !out || name != link {
		t.Errorf("ProjectOutOfRoot = %q, %v; want %q, true", name, out, link)
	}
}

func TestALinkTargetIsComparedWithTheRootAsText(t *testing.T) {
	base := physicalTemp(t)
	root := base + "/Éimg"
	writeFiles(t, root, "srv/real/proj/AGENTS.md")
	mustDo(t, os.MkdirAll(root+"/home/u/ext", 0o755))
	link := root + "/home/u/ext"
	cases := []struct {
		name   string
		target string
		want   rootScope
	}{
		{"a target spelling the root in another ASCII case is walked",
			syspath.ToNative(base + "/ÉIMG/srv/real"), rootScope{}},
		// NTFS names the same directory, but telling so would mean asking
		// the disk about a path not yet known to be inside the root.
		{"a target spelling the root with an accented letter in another case is not walked",
			syspath.ToNative(base + "/éimg/srv/real"), rootScope{name: link}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := projectScope(root, link+"/proj", rootedLinkProbes(link, testCase.target))
			if got != testCase.want {
				t.Errorf("scope %+v, want %+v", got, testCase.want)
			}
		})
	}
}

func TestALiveProjectOutOfItsHomeKeepsItsDrive(t *testing.T) {
	if got := (RootCheck{}).underRoot("C:/x/y"); got != "C:/x/y" {
		t.Errorf(`underRoot("C:/x/y") of a live run = %q, want C:/x/y`, got)
	}
	base := physicalTemp(t)
	writeFiles(t, base, "home/a.txt", "outside/proj/AGENTS.md")
	makeJunction(t, base+"/outside/proj", base+"/home/ext")
	found := projectPlace(RootCheck{}, stage.Stager{Dir: base + "/stage"}, base+"/home",
		base+"/home/ext")
	archive, _, _ := found.stager.Dest(base + "/home/ext/AGENTS.md")
	want := "fs/" + strings.ToUpper(base[:1]) + base[2:] + "/outside/proj/AGENTS.md"
	if !found.outside || archive != want {
		t.Errorf("outside %t, archive path %q, want a project out of its home archived at %q",
			found.outside, archive, want)
	}
}
