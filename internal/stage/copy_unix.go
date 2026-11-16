//go:build darwin || linux

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"golang.org/x/sys/unix"
)

// createDest creates or truncates the staged copy without following a final
// symlink, so a link recreated in the staging tree is never written through.
func createDest(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o600)
}

// createCopy creates the staged copy of the source of identity at dest
// (createDest), and the staged path as the staging tree names it (its
// place). Where the staging directory folds case (FoldsCase) the copy is
// opened without truncating, checked against where it landed (occupantOf,
// mayFill), and refused with caseCollision when another entry's copy is
// there, which is left as it was; only then emptied.
func (stager Stager) createCopy(identity [2]uint64, dest string) (*os.File, string, error) {
	if !stager.FoldsCase {
		file, err := createDest(dest)
		return file, dest, err
	}
	_, statErr := os.Lstat(dest)
	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, "", err
	}
	place := dest
	if occupant, other := stager.occupantOf(file, dest); other {
		place = occupant
	}
	if !stager.mayFill(identity, dest, place, statErr == nil) {
		_ = file.Close()
		return nil, "", caseCollision{}
	}
	if err := file.Truncate(0); err != nil {
		_ = file.Close()
		return nil, "", err
	}
	return file, place, nil
}

// checksPlaces reports whether staged places are checked: only where the
// staging directory folds case, so a case-sensitive one costs nothing.
func (stager Stager) checksPlaces() bool {
	return stager.FoldsCase
}

// handleIdentity returns the device and inode of the open source, from the
// stat CopyFile already took of its handle, in the form fsmeta.Lstat gives a
// link's (fsmeta.StatIdentity).
func handleIdentity(_ *os.File, info fs.FileInfo) [2]uint64 {
	return fsmeta.StatIdentity(info)
}

// MarkCaseSensitive does nothing: these systems have no per-directory case
// setting, and whether the staging directory folds case is asked with
// FoldsCase.
func MarkCaseSensitive(string) error {
	return nil
}

// preserve gives the copy the source's owner when running as root, its
// permission bits and its access and modification times, as cp -p does. As
// cp does when it cannot set the owner, the setuid and setgid bits are
// dropped when not running as root.
func preserve(target *os.File, dest string, info fs.FileInfo) error {
	status, ok := info.Sys().(*syscall.Stat_t)
	mode := info.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
	if os.Geteuid() == 0 && ok {
		if err := target.Chown(int(status.Uid), int(status.Gid)); err != nil {
			return err
		}
	} else {
		mode &^= fs.ModeSetuid | fs.ModeSetgid
	}
	if err := target.Chmod(mode); err != nil {
		return err
	}
	if !ok {
		return nil
	}
	times := []unix.Timespec{accessTime(status), unix.NsecToTimespec(info.ModTime().UnixNano())}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, dest, times, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &fs.PathError{Op: "utimes", Path: dest, Err: err}
	}
	return nil
}

// PhysicalDir returns dir with every symlink resolved, as `cd -P DIR && pwd
// -P` prints it, or an error when the shell's cd would fail: dir is not a
// directory or cannot be searched. A `..` is never removed as text (as
// filepath.Abs would): it goes up from where the path physically is, as the
// kernel resolves it, so `/srv/dotlink/../frank` with srv/dotlink a link to
// ../opt/escape is /opt/frank. filepath.EvalSymlinks agrees with the kernel
// on a file followed by `..`, `/` or `/.` (ENOTDIR), but it never asks for
// the right to search a directory before taking its `..`, so the kernel's
// own stat of dir is asked first: `noexec/..` fails there (EACCES).
func PhysicalDir(dir string) (string, error) {
	absolute := dir
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		absolute = cwd + "/" + dir
	}
	if _, err := os.Stat(absolute); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	var status unix.Stat_t
	if err := unix.Lstat(resolved, &status); err != nil {
		return "", &fs.PathError{Op: "lstat", Path: resolved, Err: err}
	}
	if status.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", &fs.PathError{Op: "chdir", Path: dir, Err: syscall.ENOTDIR}
	}
	if err := unix.Faccessat(unix.AT_FDCWD, resolved, unix.X_OK, unix.AT_EACCESS); err != nil {
		return "", &fs.PathError{Op: "chdir", Path: dir, Err: err}
	}
	return resolved, nil
}
