//go:build darwin || linux

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// createDest creates or truncates the staged copy without following a final
// symlink, so a link recreated in the staging tree is never written through.
func createDest(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o600)
}

// createCopy creates the staged copy at dest (createDest).
func (Stager) createCopy(dest string) (*os.File, error) {
	return createDest(dest)
}

// spelledAsNamed reports true: the staging directory of these systems is
// not compared with how the disk spells it.
func (Stager) spelledAsNamed(string) bool {
	return true
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
