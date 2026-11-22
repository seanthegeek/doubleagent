package imagetest

import (
	_ "embed"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// manifestText is the Windows image's manifest, the one Build replays here.
//
//go:embed testdata/image_windows.txt
var manifestText []byte

// manifestName names the manifest in errors.
const manifestName = "testdata/image_windows.txt"

// The homes Image.Alice and Image.Bob name, below the root.
const (
	aliceHome = "Users/alice"
	bobHome   = "home/bob"
)

// allowUnprivilegedCreate is SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE,
// which lets an account without the privilege create a link where Developer
// Mode is on; x/sys/windows does not name it.
const allowUnprivilegedCreate = 0x2

// socketPathLimit is the longest socket path Windows' AF_UNIX bind takes:
// sun_path is 108 bytes, with room for the terminating NUL.
const socketPathLimit = 107

// Errors for manifest lines that only Unix can replay.
var (
	errFifoOnWindows = errors.New("a named pipe file exists only on Unix")
	errModeOnWindows = errors.New("mode applies only on Unix; use deny= here")
)

// resolvedDirectory returns directory with every component spelled as the
// disk holds it, a short 8.3 name as its long name: the runner's TEMP holds
// a short name, which collect would otherwise print in another
// spelling than the image's.
func resolvedDirectory(test testing.TB, directory string) string {
	test.Helper()
	return syspath.ToNative(fsmeta.OnDiskPath("", syspath.FromNative(directory)))
}

// makeLink creates a symbolic link, a directory symbolic link or a junction
// at path to target. Where the account may not create symbolic links it
// returns why instead of failing.
func makeLink(path, target string, kind Kind) (skipped string, err error) {
	if kind == KindJunction {
		return "", makeJunction(target, path)
	}
	flags := uint32(allowUnprivilegedCreate)
	if kind == KindDirLink {
		flags |= windows.SYMBOLIC_LINK_FLAG_DIRECTORY
	}
	link, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return "", err
	}
	destination, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return "", err
	}
	err = windows.CreateSymbolicLink(link, destination, flags)
	if err != nil {
		// Windows before 10 1703 does not know the unprivileged flag.
		err = windows.CreateSymbolicLink(link, destination, flags&^allowUnprivilegedCreate)
	}
	if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
		return "symbolic links cannot be created here: " + err.Error(), nil
	}
	return "", err
}

// makeSpecial creates a socket file at path; Windows has no named pipe file.
func makeSpecial(path string, kind Kind) error {
	if kind == KindFifo {
		return errFifoOnWindows
	}
	if len(path) <= socketPathLimit {
		return bindSocket(path)
	}
	short, err := osfile.MkdirTemp("", "sock")
	if err != nil {
		return err
	}
	defer osfile.RemoveAll(short)
	bound := filepath.Join(short, filepath.Base(path))
	if err := bindSocket(bound); err != nil {
		return err
	}
	return osfile.Rename(bound, path)
}

// bindSocket binds an AF_UNIX socket at path and closes it without removing
// the file, which keeps its reparse tag.
func bindSocket(path string) error {
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	listener.SetUnlinkOnClose(false)
	return listener.Close()
}

// finish sets the entry's access and modification times to its manifest
// time on the entry itself, not following a link, then adds its deny rule.
// It returns what takes the rule away again, or nil when there is none.
func finish(path string, entry Entry) (undo func() error, err error) {
	if hasMode(entry.Kind) && entry.Mode != defaultMode(entry.Kind) {
		return nil, errModeOnWindows
	}
	if err := setEntryTime(path, entry.ModTime); err != nil {
		return nil, err
	}
	if entry.Deny == "" {
		return nil, nil
	}
	return denyAccess(path, entry.Deny)
}

// setEntryTime sets the access and write times of the entry at path to when,
// through a handle opened on the entry itself (a link or junction is not
// followed; a directory needs backup semantics to be opened at all).
func setEntryTime(path string, when time.Time) error {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	stamp := windows.NsecToFiletime(when.UnixNano())
	return windows.SetFileTime(handle, nil, &stamp, &stamp)
}
