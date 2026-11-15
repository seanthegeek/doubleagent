//go:build darwin || linux

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// openSource opens path for reading without following a final symlink.
// O_NONBLOCK keeps the open from waiting on a FIFO that replaced the file
// after its lstat; CopyFile then refuses it as not a regular file.
func openSource(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// createDest creates or truncates the staged copy without following a final
// symlink, so a link recreated in the staging tree is never written through.
func createDest(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o600)
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
// directory or cannot be searched.
func PhysicalDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
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
