// Package osfile wraps the os functions that take a path so that, on Windows,
// each one reaches exactly the entry it names. Windows drops a dot or a space
// ending a component of an ordinary path, yet NTFS stores such names: opened
// that way, a directory "x." is not found or, beside a neighbour "x", the
// neighbour is opened without an error, and a collector would record one
// entry's name and metadata with another's contents and hash. A path with the
// \\?\ prefix is used as written, and os adds it only to a long path, so every
// function here calls os on syspath.Extended(path) and returns errors with
// native paths. A returned file's Name and its methods' errors keep the
// prefix. On Unix each function is its os counterpart.
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

// Create creates or truncates path, as os.Create does.
func Create(path string) (*os.File, error) {
	file, err := os.Create(syspath.Extended(path))
	return file, syspath.NativeErrors(err)
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

// MkdirAll creates the directory path and its missing parents, as
// os.MkdirAll does. On Windows os.MkdirAll walks the extended form's parents
// down to its \\?\X: volume and stats each, so it takes the form whole.
func MkdirAll(path string, perm fs.FileMode) error {
	return syspath.NativeErrors(os.MkdirAll(syspath.Extended(path), perm))
}

// Remove removes the file or empty directory path, as os.Remove does.
func Remove(path string) error {
	return syspath.NativeErrors(os.Remove(syspath.Extended(path)))
}

// RemoveAll removes path and everything under it, as os.RemoveAll does.
func RemoveAll(path string) error {
	return syspath.NativeErrors(os.RemoveAll(syspath.Extended(path)))
}

// Chtimes sets the access and modification times of path, following a link,
// as os.Chtimes does.
func Chtimes(path string, accessed, modified time.Time) error {
	return syspath.NativeErrors(os.Chtimes(syspath.Extended(path), accessed, modified))
}
