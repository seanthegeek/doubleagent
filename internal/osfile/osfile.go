// Package osfile wraps the os functions that take a path, so that on Windows
// every file operation names exactly the entry it was given.
//
// Windows normalises an ordinary path such as C:\dir\name before using it: a
// dot or a space ending a path component is dropped. NTFS itself stores such
// names, and other software can create them. Opened through an ordinary path,
// a directory named "x." is therefore not found, or, when a neighbour named
// "x" exists, the neighbour is opened in its place without an error. A
// collector that listed "x." and then read "x" would record one entry's name
// and metadata beside another entry's contents and hash.
//
// A path with the \\?\ prefix is passed to the file system as written, with no
// normalisation, so it reaches the entry it names; the same form lifts the
// 260-character path limit. The os package adds that prefix only to a long
// path. Every function here calls its os counterpart on syspath.Extended(path),
// which adds the prefix to any absolute path, so listing, describing, reading
// and writing all agree on which entry a path means. Code built for Windows
// calls these functions and never the os ones; a test in this package fails
// on a direct call.
//
// Errors come back with their paths in native form, without the prefix. A
// returned *os.File keeps the extended form as its Name, and errors from its
// methods are not rewritten: a caller that reports one passes it through
// syspath.NativeErrors. On Unix a path is used as written, so each function
// is its os counterpart.
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
