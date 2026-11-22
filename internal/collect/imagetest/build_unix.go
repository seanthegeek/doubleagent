//go:build unix

package imagetest

import (
	_ "embed"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// manifestText is the Unix image's manifest, the one Build replays here.
//
//go:embed testdata/image_unix.txt
var manifestText []byte

// manifestName names the manifest in errors.
const manifestName = "testdata/image_unix.txt"

// The homes Image.Alice and Image.Bob name, below the root.
const (
	aliceHome = "home/alice"
	bobHome   = "Users/bob"
)

// socketPathLimit is the longest socket path bind(2) takes on every Unix the
// tests run on: sun_path is 104 bytes on macOS and the BSDs, 108 on Linux,
// with room for the terminating NUL.
const socketPathLimit = 103

// errDenyOnUnix is the error for a deny attribute, which only Windows has.
var errDenyOnUnix = errors.New("deny applies only on Windows; use mode= here")

// resolvedDirectory returns directory with its links resolved, as the
// collect resolves -r: on macOS the temporary directory lies under /var,
// a link to /private/var.
func resolvedDirectory(test testing.TB, directory string) string {
	test.Helper()
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		test.Fatal(err)
	}
	return resolved
}

// settle does nothing: a Unix directory listing reports what the entry holds.
func settle(testing.TB, string) {}

// Settle does nothing here; on Windows it settles a tree a test changed.
func Settle(testing.TB, string) {}

// makeLink creates a symbolic link at path to target; Unix has one kind.
func makeLink(path, target string, kind Kind) (skipped string, err error) {
	if kind != KindLink {
		return "", errors.New(string(kind) + " exists only on Windows")
	}
	return "", os.Symlink(target, path)
}

// makeSpecial creates a named pipe or a socket file at path.
func makeSpecial(path string, kind Kind) error {
	if kind == KindFifo {
		return syscall.Mkfifo(path, 0o644)
	}
	return makeSocket(path)
}

// makeSocket binds a Unix domain socket at path and closes it, leaving the
// file. A path too long for bind is reached through a short link to its
// directory (shortLinkTo): bind follows the link and creates the socket
// where the path names it.
func makeSocket(path string) error {
	if len(path) <= socketPathLimit {
		return bindSocket(path)
	}
	for _, parent := range []string{os.TempDir(), shortTempDir} {
		through, remove, err := shortLinkTo(parent, path)
		if err != nil {
			return err
		}
		if len(through) <= socketPathLimit {
			defer remove()
			return bindSocket(through)
		}
		remove()
	}
	return errors.New("the socket path " + path + " is too long for bind even through a link in " +
		shortTempDir)
}

// shortTempDir is the system's own temporary directory, short on every
// Unix, for a link when TMPDIR names a long one.
const shortTempDir = "/tmp"

// shortLinkTo makes a new private directory in parent holding a link to
// path's directory, and returns path's name through that link and what
// removes the directory again. The directory's name is random and made
// with O_EXCL (os.MkdirTemp), so no other account can take its place.
func shortLinkTo(parent, path string) (string, func(), error) {
	short, err := os.MkdirTemp(parent, "sock")
	if err != nil {
		return "", nil, err
	}
	remove := func() { os.RemoveAll(short) }
	link := filepath.Join(short, "d")
	if err := os.Symlink(filepath.Dir(path), link); err != nil {
		remove()
		return "", nil, err
	}
	return filepath.Join(link, filepath.Base(path)), remove, nil
}

// bindSocket binds a socket at path and closes it without removing the file.
func bindSocket(path string) error {
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	listener.SetUnlinkOnClose(false)
	return listener.Close()
}

// setEntryTime sets the access and modification times of the entry at path
// to when, on the link itself for a link.
func setEntryTime(path string, when time.Time) error {
	stamp := unix.NsecToTimespec(when.UnixNano())
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, []unix.Timespec{stamp, stamp}, unix.AT_SYMLINK_NOFOLLOW)
}

// finish sets the entry's access and modification times to its manifest
// time, on the link itself for a link, then its mode. For a directory whose
// mode shuts out its owner it returns what opens it again, so the tree can
// be removed; otherwise nil.
func finish(path string, entry Entry) (undo func() error, err error) {
	if entry.Deny != "" {
		return nil, errDenyOnUnix
	}
	if err := setEntryTime(path, entry.ModTime); err != nil {
		return nil, err
	}
	if !hasMode(entry.Kind) {
		return nil, nil
	}
	if err := os.Chmod(path, entry.Mode); err != nil {
		return nil, err
	}
	if entry.Kind == KindDir && entry.Mode&0o700 != 0o700 {
		return func() error { return os.Chmod(path, 0o700) }, nil
	}
	return nil, nil
}
