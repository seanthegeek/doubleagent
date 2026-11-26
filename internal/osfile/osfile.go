package osfile

import (
	"io/fs"
	"os"
	"time"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Open opens path for reading, as os.Open does.
func Open(path string) (*os.File, error) {
	file, err := os.Open(syspath.Extended(path))
	return file, syspath.NativeErrors(err)
}

// OpenFile opens path with flag and perm, as os.OpenFile does.
func OpenFile(path string, flag int, perm fs.FileMode) (*os.File, error) {
	file, err := os.OpenFile(syspath.Extended(path), flag, perm)
	return file, syspath.NativeErrors(err)
}

// CreateNew creates the file at path for writing, new: anything already at
// path, a link or a dangling link included, is an error and is neither
// followed nor changed (O_EXCL; on Windows CREATE_NEW with
// FILE_FLAG_OPEN_REPARSE_POINT, syscall/syscall_windows.go). Every file a
// run writes in its output is made this way, so another account that can
// write the output directory cannot make the run write through a link it
// planted there.
func CreateNew(path string) (*os.File, error) {
	return OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
}

// Lstat describes path itself, a link included, as os.Lstat does.
func Lstat(path string) (fs.FileInfo, error) {
	info, err := os.Lstat(syspath.Extended(path))
	return info, syspath.NativeErrors(err)
}

// Stat describes path, following a link, as os.Stat does.
func Stat(path string) (fs.FileInfo, error) {
	info, err := os.Stat(syspath.Extended(path))
	return info, syspath.NativeErrors(err)
}

// Readlink returns the stored target of the link path, as os.Readlink does;
// the target is returned verbatim.
func Readlink(path string) (string, error) {
	target, err := os.Readlink(syspath.Extended(path))
	return target, syspath.NativeErrors(err)
}

// ReadDir lists the directory path in directory order, as
// (*os.File).ReadDir(-1) gives it, never sorted. A listing that fails part
// way returns the entries read so far with the error.
func ReadDir(path string) ([]os.DirEntry, error) {
	directory, err := Open(path)
	if err != nil {
		return nil, err
	}
	entries, err := directory.ReadDir(-1)
	if closeError := directory.Close(); err == nil {
		err = closeError
	}
	return entries, syspath.NativeErrors(err)
}

// ReadFile returns the contents of path, as os.ReadFile does.
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(syspath.Extended(path))
	return data, syspath.NativeErrors(err)
}

// WriteFile writes data to path, creating or truncating it, as os.WriteFile
// does.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	return syspath.NativeErrors(os.WriteFile(syspath.Extended(path), data, perm))
}

// Mkdir creates the directory path, as os.Mkdir does: anything already at
// path, a link or a dangling link included, is an error and is neither
// followed nor changed (mkdir(2) returns EEXIST; on Windows CreateDirectory
// fails with ERROR_ALREADY_EXISTS).
func Mkdir(path string, perm fs.FileMode) error {
	return syspath.NativeErrors(os.Mkdir(syspath.Extended(path), perm))
}

// MkdirAll creates the directory path and its missing parents, as
// os.MkdirAll does. On Windows os.MkdirAll walks the extended form's parents
// down to its \\?\X: volume and stats each, so it takes the form whole.
func MkdirAll(path string, perm fs.FileMode) error {
	return syspath.NativeErrors(os.MkdirAll(syspath.Extended(path), perm))
}

// MkdirTemp creates a new directory in dir, which must exist, with a name
// that begins with pattern's text before its last `*` and holds random
// characters in the star's place, as os.MkdirTemp does, and returns its path
// in the native form. Only the owner can read or enter it.
func MkdirTemp(dir, pattern string) (string, error) {
	path, err := os.MkdirTemp(syspath.Extended(dir), pattern)
	if err != nil {
		return "", syspath.NativeErrors(err)
	}
	return syspath.ToNative(syspath.FromNative(path)), nil
}

// Remove removes the file or empty directory path, as os.Remove does.
func Remove(path string) error {
	return syspath.NativeErrors(os.Remove(syspath.Extended(path)))
}

// Rename renames oldPath to newPath, as os.Rename does, both in extended
// form. An existing file at newPath loses the name and is never written
// into: rename(2) replaces the name in one step, so a hard link to that
// file keeps its content and a symbolic link there is itself replaced;
// Windows MoveFileEx with MOVEFILE_REPLACE_EXISTING does the same
// (internal/syscall/windows/syscall_windows.go), failing while another
// program holds the file open without sharing deletion. A directory at
// newPath is an error on both (os/file.go, os/file_unix.go). The error is an
// *os.LinkError with both names in native form.
func Rename(oldPath, newPath string) error {
	return syspath.NativeErrors(os.Rename(syspath.Extended(oldPath), syspath.Extended(newPath)))
}

// RemoveAll removes path and everything under it, as os.RemoveAll does.
func RemoveAll(path string) error {
	return syspath.NativeErrors(os.RemoveAll(syspath.Extended(path)))
}

// OpenRoot opens the directory path as an os.Root, as os.OpenRoot does. The
// root's methods take names relative to it and are not wrapped. On Windows
// each name is cleaned by GetFullPathName first (os/root_windows.go), which
// drops a dot or a space ending a name and refuses a device name, and every
// open asks for backup intent (FILE_OPEN_FOR_BACKUP_INTENT,
// internal/syscall/windows/at_windows.go), under which the system may grant
// access before checking the access list (NtCreateFile). So on Windows no
// collected name is given to one, and nothing whose answer must match an
// ordinary creation uses one.
func OpenRoot(path string) (*os.Root, error) {
	root, err := os.OpenRoot(syspath.Extended(path))
	return root, syspath.NativeErrors(err)
}

// Chtimes sets the access and modification times of path, following a link,
// as os.Chtimes does.
func Chtimes(path string, accessed, modified time.Time) error {
	return syspath.NativeErrors(os.Chtimes(syspath.Extended(path), accessed, modified))
}
