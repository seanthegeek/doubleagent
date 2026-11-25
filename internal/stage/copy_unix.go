//go:build darwin || linux

package stage

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"golang.org/x/sys/unix"
)

// createDest creates the staged copy at name in the tree exclusively,
// never following a final symlink: when the volume takes the name for an
// existing entry's, by any rule it has, the create fails and nothing is
// opened for write.
func createDest(tree *Tree, name string) (createdFile, error) {
	file, err := tree.CreateNew(name, 0o600)
	if err != nil {
		return createdFile{}, err
	}
	created := createdFile{file: file, place: reportedPath(file)}
	if info, err := file.Stat(); err == nil {
		created.staged = fsmeta.StatIdentity(info)
	}
	return created, nil
}

// handleIdentity returns the device and inode of the open source, from the
// stat Copy already took of its handle, in the form fsmeta.Lstat gives a
// link's (fsmeta.StatIdentity).
func handleIdentity(_ *os.File, info fs.FileInfo) [2]uint64 {
	return fsmeta.StatIdentity(info)
}

// MarkCaseSensitive does nothing: these systems have no per-directory case
// setting, and whether the staging directory takes two names for one is
// asked with FoldsCase.
func MarkCaseSensitive(string) error {
	return nil
}

// preserve gives the copy at name in the tree the source's owner when
// running as root, its permission bits and its access and modification
// times, as cp -p does. As cp does when it cannot set the owner, the setuid
// and setgid bits are dropped when not running as root.
func preserve(target *os.File, tree *Tree, name string, info fs.FileInfo) error {
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
	accessed := accessTime(status)
	return tree.SetTimes(name, time.Unix(accessed.Sec, accessed.Nsec), info.ModTime())
}

// PhysicalDir returns dir with every symlink resolved, as `cd -P DIR && pwd
// -P` prints it, or an error when the shell's cd would fail: dir is not a
// directory or cannot be searched. A `..` goes up from where the path
// physically is, as the kernel resolves it, never removed as text (as
// filepath.Abs would): `/srv/dotlink/../frank`, with srv/dotlink a link to
// ../opt/escape, is /opt/frank. filepath.EvalSymlinks agrees with the
// kernel except that it never asks for the right to search a directory
// before taking its `..`, so the kernel's own stat of dir comes first:
// `noexec/..` fails there (EACCES).
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
