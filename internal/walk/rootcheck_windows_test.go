package walk

import (
	"errors"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// makeSymlink makes link a symbolic link holding target as written, a
// directory link when directory is set, or skips the test where the account
// may not create one.
func makeSymlink(t *testing.T, target, link string, directory bool) {
	t.Helper()
	// SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE, which x/sys does not
	// name: developer mode lets a plain account create links with it.
	flags := uint32(0x2)
	if directory {
		flags |= windows.SYMBOLIC_LINK_FLAG_DIRECTORY
	}
	linkName, err := windows.UTF16PtrFromString(syspath.ToNative(link))
	mustDo(t, err)
	targetName, err := windows.UTF16PtrFromString(target)
	mustDo(t, err)
	err = windows.CreateSymbolicLink(linkName, targetName, flags)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skipf("creating a symbolic link needs a privilege here: %v", err)
	}
	mustDo(t, err)
}

// finalPath returns, in canonical form, the path Windows reports for the
// object an open of path reaches, path opened as the binary opens it
// (syspath.Extended), or the error when the open fails.
func finalPath(path string) (string, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|
		windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil {
		return "", err
	}
	return syspath.FromNative(windows.UTF16ToString(buffer[:n])), nil
}

// measuredImage builds, under root, the links whose targets hold `..`: j a
// junction to real/a; in real/a a directory link l to ..\x, a file link f to
// ..\y.txt and a directory link t to nope\..\..\x; k a junction whose
// absolute target is real\a\..\x; s1 a relative and as an absolute
// directory link to real/a. Marker files tell which directory an open lands
// in.
func measuredImage(t *testing.T, root string) {
	t.Helper()
	writeFiles(t, root, "x/m.txt", "real/x/m.txt", "y.txt", "real/y.txt", "real/a/keep")
	makeJunction(t, root+"/real/a", root+"/j")
	makeJunction(t, root+"/real/a/../x", root+"/k")
	makeSymlink(t, `..\x`, root+"/real/a/l", true)
	makeSymlink(t, `..\y.txt`, root+"/real/a/f", false)
	makeSymlink(t, `nope\..\..\x`, root+"/real/a/t", true)
	makeSymlink(t, `real\a`, root+"/s1", true)
	makeSymlink(t, syspath.ToNative(root+"/real/a"), root+"/as", true)
}

func TestTheRootCheckFollowsLinksAsWindowsDoes(t *testing.T) {
	root := physicalTemp(t)
	measuredImage(t, root)
	check := NewRootCheck(root)
	for _, path := range []string{"/j/l/m.txt", "/j/f", "/j/t/m.txt", "/k/m.txt", "/real/a/l/m.txt",
		"/s1/l/m.txt", "/as/l/m.txt", "/j/../j/l/m.txt"} {
		t.Run(path, func(t *testing.T) {
			full := root + path
			found, reached := check.resolve(full, imageProbes())
			opened, err := finalPath(full)
			t.Logf("Windows opened %q (error %v); the check reached %q (%+v)", opened, err, reached,
				found)
			if err != nil {
				return
			}
			if found.name != "" && !found.inImage {
				t.Errorf("the check refused %s, which Windows opens at %s inside the root", full, opened)
				return
			}
			if !syspath.Equal(reached, opened) {
				t.Errorf("the check reached %s for %s, Windows opened %s", reached, full, opened)
			}
		})
	}
}

func TestTheRootCheckAndWindowsAgreeOnNastyPaths(t *testing.T) {
	root, outside := physicalTemp(t), physicalTemp(t)
	writeFiles(t, root, "Users/alice/a/b/f", "Users/alice/keep")
	writeFiles(t, outside, "x", "secret/x")
	makeJunction(t, outside, root+"/Users/alice/j")
	makeJunction(t, outside+"/secret", root+"/Users/alice/secret")
	makeSymlink(t, `a\b`, root+"/Users/alice/link", true)
	check := NewRootCheck(root)
	for _, path := range []string{
		"/Users/alice/nope/../j/x", "/Users/alice/link/../secret/x", "/Users/alice/j./x",
		"/Users/alice/j /x", "/Users/alice//a//b/f", "/Users/alice/a/./b/.", "/Users/../../..",
		"/Users/alice/link/f", "/Users/alice/a/b/../../j/x", "/Users/alice/link/../../alice/j/x",
	} {
		t.Run(path, func(t *testing.T) {
			full := root + path
			if _, leaves := check.Exit(full); leaves {
				return
			}
			opened, err := finalPath(full)
			if err != nil {
				return
			}
			if _, inside := stage.Under(opened, root); !inside {
				t.Errorf("the check let %s through, and Windows opens %s, outside the root", full,
					opened)
			}
		})
	}
}
