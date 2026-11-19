package walk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/seanthegeek/doubleagent/internal/follow"

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

// measuredImage builds, under base/img (returned), the links whose targets
// hold `..`: j a junction to real/a; in real/a a directory link l to ..\x, a
// file link f to ..\y.txt and a directory link t to nope\..\..\x; k a
// junction whose absolute target is real\a\..\x; s1 a relative and as an
// absolute directory link to real/a; real/s a link to deep, real/deep/a/l a
// link to ..\..\x, j2 a junction to real\s\a (through the link s), and j1 a
// junction to nest, where nest/j2 is that junction again. Marker files tell
// which directory an open lands in, base/x/m.txt outside the image.
func measuredImage(t *testing.T, base string) string {
	t.Helper()
	root := base + "/img"
	writeFiles(t, base, "x/m.txt")
	writeFiles(t, root, "x/m.txt", "real/x/m.txt", "y.txt", "real/y.txt", "real/a/keep",
		"real/deep/a/keep", "real/deep/x/m.txt", "nest/keep")
	makeJunction(t, root+"/real/a", root+"/j")
	makeJunction(t, root+"/real/a/../x", root+"/k")
	makeSymlink(t, `..\x`, root+"/real/a/l", true)
	makeSymlink(t, `..\y.txt`, root+"/real/a/f", false)
	makeSymlink(t, `nope\..\..\x`, root+"/real/a/t", true)
	makeSymlink(t, `real\a`, root+"/s1", true)
	makeSymlink(t, syspath.ToNative(root+"/real/a"), root+"/as", true)
	makeSymlink(t, `deep`, root+"/real/s", true)
	makeSymlink(t, `..\..\x`, root+"/real/deep/a/l", true)
	makeJunction(t, root+"/real/s/a", root+"/j2")
	makeJunction(t, root+"/real/s/a", root+"/nest/j2")
	makeJunction(t, root+"/nest", root+"/j1")
	return root
}

func TestTheRootCheckFollowsLinksAsWindowsDoes(t *testing.T) {
	base := physicalTemp(t)
	root := measuredImage(t, base)
	check := NewRootCheck(root)
	// Each path opens the file named, relative to the image root (".." for
	// base, outside it), or is refused with the kind and link named.
	cases := []struct {
		path, opens string
		kind        follow.Kind
		link        string
	}{
		{path: "/j/l/m.txt", opens: "x/m.txt"},
		{path: "/j/f", opens: "y.txt"},
		{path: "/j/t/m.txt", opens: "x/m.txt"},
		{path: "/k/m.txt", kind: follow.Unverified, link: "k"},
		{path: "/real/a/l/m.txt", opens: "real/x/m.txt"},
		{path: "/s1/l/m.txt", opens: "real/x/m.txt"},
		{path: "/as/l/m.txt", opens: "real/x/m.txt"},
		{path: "/j/../j/l/m.txt", opens: "x/m.txt"},
		{path: "/real/s/a/l/m.txt", opens: "real/x/m.txt"},
		// The link is named where it physically is, through the junction.
		{path: "/j2/l/m.txt", kind: follow.LeadsOut, link: "real/deep/a/l"},
		{path: "/j1/j2/l/m.txt", opens: "x/m.txt"},
	}
	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			full := root + testCase.path
			found, reached := check.resolve(full, imageProbes())
			opened, err := finalPath(full)
			t.Logf("Windows opened %q (error %v); the check reached %q (%+v)", opened, err, reached,
				found)
			refusal, refused := check.Exit(full)
			if testCase.opens == "" {
				want := follow.Refusal{Link: root + "/" + testCase.link, Kind: testCase.kind}
				if !refused || refusal != want {
					t.Errorf("Exit = %+v, %v; want %+v", refusal, refused, want)
				}
				return
			}
			want := root + "/" + testCase.opens
			if err != nil || !syspath.Equal(opened, want) {
				t.Errorf("Windows opened %q (error %v), want %s", opened, err, want)
			}
			if refused || !syspath.Equal(reached, want) {
				t.Errorf("the check reached %q (refused %+v, %v), want %s", reached, refusal,
					refused, want)
			}
		})
	}
	// Where Windows opens the leading-out case, outside the image.
	if opened, err := finalPath(root + "/j2/l/m.txt"); err == nil &&
		!syspath.Equal(opened, base+"/x/m.txt") {
		t.Errorf("Windows opened j2/l/m.txt at %s, not %s/x/m.txt", opened, base)
	}
}

// Lone surrogates, which Go names in WTF-8.
const loneD800, loneD801 = "\xed\xa0\x80", "\xed\xa1\x80"

// nastyPathsImage builds an image root with links and names the root check
// must refuse or pass: junctions and links out of the root, links to the
// system volume by names os.Readlink does not turn into a drive path, and
// lone-surrogate names. It returns the image root.
func nastyPathsImage(t *testing.T) string {
	t.Helper()
	root, outside := physicalTemp(t), physicalTemp(t)
	writeFiles(t, root, "Users/alice/a/b/f", "Users/alice/keep")
	writeFiles(t, outside, "x", "secret/x")
	makeJunction(t, outside, root+"/Users/alice/j")
	makeJunction(t, outside+"/secret", root+"/Users/alice/secret")
	makeSymlink(t, `a\b`, root+"/Users/alice/link", true)
	volume, device := systemVolumeNames(t)
	// Links to the system volume by names os.Readlink does not turn into a
	// drive path: its volume GUID name, the same volume through GLOBALROOT,
	// and, stored raw without the relative flag, its device name and the
	// global namespace. The check must refuse every one as unverified.
	makeSymlink(t, volume, root+"/Users/alice/vol", true)
	makeSymlink(t, `\\?\GLOBALROOT`+device+`\`, root+"/Users/alice/glob", true)
	storeSymlink(t, device+`\`, root+"/Users/alice/dev")
	storeSymlink(t, `\GLOBAL??\`+systemDrive()+`\`, root+"/Users/alice/global")
	// Lone surrogates, which Go names in WTF-8: a junction named U+D800 to
	// the Windows directory, a link L whose stored target is that one
	// unit, and a directory named U+D801 holding a file inside the image.
	// Decoding L's target to U+FFFD would have the walk look for another
	// name than the one Windows follows.
	makeJunction(t, systemDrive()+"/Windows", root+"/Users/alice/"+loneD800)
	makeSymlink(t, loneD800, root+"/Users/alice/L", true)
	writeFiles(t, root, "Users/alice/"+loneD801+"/f")
	return root
}

func TestTheRootCheckAndWindowsAgreeOnNastyPaths(t *testing.T) {
	root := nastyPathsImage(t)
	check := NewRootCheck(root)
	t.Run("a link to a lone surrogate", func(t *testing.T) {
		full := root + "/Users/alice/L/win.ini"
		opened, err := finalPath(full)
		t.Logf("Windows opens %s at %q (error %v)", full, opened, err)
		want := follow.Refusal{Link: root + "/Users/alice/L", Kind: follow.Unverified}
		if got, leaves := check.Exit(full); !leaves || got != want {
			t.Errorf("Exit = %+v, %v; want %+v, true", got, leaves, want)
		}
	})
	t.Run("a directory named by a lone surrogate", func(t *testing.T) {
		full := root + "/Users/alice/" + loneD801 + "/f"
		opened, err := finalPath(full)
		if err != nil || !syspath.Equal(opened, full) {
			t.Errorf("Windows opened %q (error %v), want %q", opened, err, full)
		}
		if refusal, leaves := check.Exit(full); leaves {
			t.Errorf("Exit refused %+v", refusal)
		}
	})
	for _, link := range []string{"vol", "glob", "dev", "global"} {
		t.Run(link, func(t *testing.T) {
			full := root + "/Users/alice/" + link + "/Windows/win.ini"
			opened, err := finalPath(full)
			t.Logf("Windows opens %s at %q (error %v)", full, opened, err)
			want := follow.Refusal{Link: root + "/Users/alice/" + link, Kind: follow.Unverified}
			if got, leaves := check.Exit(full); !leaves || got != want {
				t.Errorf("Exit(%s) = %+v, %v; want %+v, true", full, got, leaves, want)
			}
		})
	}
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

// systemDrive returns the drive Windows is installed on, `C:`.
func systemDrive() string {
	return os.Getenv("SystemDrive")
}

// systemVolumeNames returns the system volume's GUID name
// (`\\?\Volume{...}\`) and its device name (`\Device\HarddiskVolumeN`).
func systemVolumeNames(t *testing.T) (volume, device string) {
	t.Helper()
	drive := systemDrive()
	if len(drive) != 2 {
		t.Fatalf("SystemDrive is %q", drive)
	}
	mountPoint, err := windows.UTF16PtrFromString(drive + `\`)
	mustDo(t, err)
	buffer := make([]uint16, windows.MAX_PATH)
	mustDo(t, windows.GetVolumeNameForVolumeMountPoint(mountPoint, &buffer[0], uint32(len(buffer))))
	volume = windows.UTF16ToString(buffer)
	name, err := windows.UTF16PtrFromString(drive)
	mustDo(t, err)
	_, err = windows.QueryDosDevice(name, &buffer[0], uint32(len(buffer)))
	mustDo(t, err)
	device = windows.UTF16ToString(buffer)
	if !strings.HasPrefix(volume, `\\?\Volume{`) || !strings.HasPrefix(device, `\Device\`) {
		t.Fatalf("system volume names %q and %q", volume, device)
	}
	return volume, device
}

// storeSymlink makes link a directory symbolic link whose reparse data holds
// substitute exactly, without the relative flag, written with
// FSCTL_SET_REPARSE_POINT so that no API rewrites it first.
func storeSymlink(t *testing.T, substitute, link string) {
	t.Helper()
	mustDo(t, os.Mkdir(syspath.ToNative(link), 0o755))
	name, err := windows.UTF16PtrFromString(syspath.ToNative(link))
	mustDo(t, err)
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	mustDo(t, err)
	defer windows.CloseHandle(handle)
	text := utf16.Encode([]rune(substitute))
	size := uint16(2 * len(text))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_SYMLINK)
	data = binary.LittleEndian.AppendUint16(data, 12+2*size)
	data = binary.LittleEndian.AppendUint16(data, 0)
	for _, word := range []uint16{0, size, size, size} {
		data = binary.LittleEndian.AppendUint16(data, word)
	}
	data = binary.LittleEndian.AppendUint32(data, 0)
	for _, unit := range append(text, text...) {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		t.Skipf("storing a symbolic link needs a privilege here: %v", err)
	}
	mustDo(t, err)
}

func TestALinkToAnotherCaseOfTheRootIsJudgedByTheDisk(t *testing.T) {
	for _, caseSensitive := range []bool{false, true} {
		t.Run(fmt.Sprintf("case-sensitive %v", caseSensitive), func(t *testing.T) {
			base := physicalTemp(t)
			if caseSensitive {
				output, err := exec.Command("fsutil", "file", "setCaseSensitiveInfo",
					syspath.ToNative(base), "enable").CombinedOutput()
				if err != nil {
					t.Skipf("fsutil cannot make %s case-sensitive here: %v: %s", base, err,
						strings.TrimSpace(string(output)))
				}
			}
			root := base + "/img"
			writeFiles(t, root, "x/m.txt")
			if caseSensitive {
				writeFiles(t, base, "IMG/x/m.txt")
			}
			// An absolute link whose target spells the root IMG.
			makeSymlink(t, syspath.ToNative(base+"/IMG/x"), root+"/l", true)
			check := NewRootCheck(root)
			full := root + "/l/m.txt"
			opened, err := finalPath(full)
			t.Logf("Windows opens %s at %q (error %v)", full, opened, err)
			refusal, refused := check.Exit(full)
			if caseSensitive {
				want := follow.Refusal{Link: root + "/l", Kind: follow.LeadsOut}
				if !refused || refusal != want {
					t.Errorf("Exit = %+v, %v; want %+v: IMG is another directory", refusal,
						refused, want)
				}
				return
			}
			if refused || err != nil || !syspath.Equal(opened, root+"/x/m.txt") {
				t.Errorf("Exit = %+v, %v; Windows opened %q (%v); want the root's own x/m.txt",
					refusal, refused, opened, err)
			}
		})
	}
}

// otherShareCase returns a UNC path with its server and share names in the
// other case of each ASCII letter, the rest unchanged.
func otherShareCase(path string) string {
	volume := volumeOf(path)
	swapped := []byte(volume)
	for i, char := range swapped {
		switch {
		case 'a' <= char && char <= 'z':
			swapped[i] = char - 'a' + 'A'
		case 'A' <= char && char <= 'Z':
			swapped[i] = char - 'A' + 'a'
		}
	}
	return string(swapped) + path[len(volume):]
}

// otherDriveCase returns path with its drive letter in the other case.
func otherDriveCase(path string) string {
	if len(path) < 2 || path[1] != ':' {
		return path
	}
	letter := path[0]
	if 'a' <= letter && letter <= 'z' {
		return string(letter-'a'+'A') + path[1:]
	}
	return string(letter-'A'+'a') + path[1:]
}

// TestTheRootCheckIgnoresTheCaseOfTheVolume is the table of the volume part
// of a root and of a path: the root's drive letter as Windows reports it or
// in the other case, a drive root, a UNC root; the path under it spelled
// with either case of the letter (or of the server and share), and an
// absolute link target spelled with the other case. Windows ignores the case
// of a drive letter and of server and share names, and so must the check.
func TestTheRootCheckIgnoresTheCaseOfTheVolume(t *testing.T) {
	root := physicalTemp(t)
	writeFiles(t, root, "a/f")
	makeSymlink(t, syspath.ToNative(otherDriveCase(root)+"/a"), root+"/l", true)
	roots := []string{root, otherDriveCase(root)}
	share := "//localhost/" + root[:1] + "$" + root[2:]
	if _, err := os.Stat(syspath.ToNative(share)); err == nil {
		roots = append(roots, share, "//LOCALHOST/"+strings.ToLower(root[:1])+"$"+root[2:])
	} else {
		t.Logf("no administrative share here (%v): the UNC rows are not run", err)
	}
	for _, given := range roots {
		check := NewRootCheck(given)
		paths := []string{given + "/a/f", otherDriveCase(given) + "/a/f"}
		if strings.HasPrefix(given, "//") {
			// The same directory through the share spelled in the other
			// case of the server and share names.
			paths = append(paths, otherShareCase(given)+"/a/f")
			// l's target is a drive path. The administrative share is the
			// same disk, but the check cannot connect a drive path to the
			// share, so it refuses the link as leading out; Windows does
			// not follow a link from a share to a local path by default.
			want := follow.Refusal{Link: given + "/l", Kind: follow.LeadsOut}
			if refusal, refused := check.Exit(given + "/l/f"); !refused || refusal != want {
				t.Errorf("NewRootCheck(%s).Exit(%s/l/f) = %+v, %v; want %+v", given, given,
					refusal, refused, want)
			}
		} else {
			paths = append(paths, given+"/l/f")
		}
		for _, path := range paths {
			t.Run(given+" "+path, func(t *testing.T) {
				if refusal, refused := check.Exit(path); refused {
					t.Errorf("NewRootCheck(%s).Exit(%s) refused %+v", given, path, refusal)
				}
				_, reached := check.resolve(path, imageProbes())
				if !syspath.Equal(reached, root+"/a/f") && !strings.HasPrefix(given, "//") {
					t.Errorf("reached %q, want %s/a/f", reached, root)
				}
			})
		}
	}
	// A drive root given with its letter in lower case: a path under it is
	// inside, whichever case spells the letter.
	drive := strings.ToLower(systemDrive())
	check := NewRootCheck(drive + "/")
	for _, path := range []string{drive + "/Windows/win.ini", strings.ToUpper(drive) + "/Windows/win.ini"} {
		if refusal, refused := check.Exit(path); refused {
			t.Errorf("NewRootCheck(%s/).Exit(%s) refused %+v", drive, path, refusal)
		}
	}
}
