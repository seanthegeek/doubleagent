package reason

import (
	"errors"
	"io/fs"
	"strings"
	"syscall"
	"unicode/utf16"
)

// Windows error codes the syscall package does not name, from the system
// error code list of the Win32 documentation.
const (
	errorInvalidFunction     syscall.Errno = 1    // ERROR_INVALID_FUNCTION
	errorPathNotFound        syscall.Errno = 3    // ERROR_PATH_NOT_FOUND
	errorInvalidName         syscall.Errno = 123  // ERROR_INVALID_NAME
	errorFilenameExcedRange  syscall.Errno = 206  // ERROR_FILENAME_EXCED_RANGE
	errorDirectory           syscall.Errno = 267  // ERROR_DIRECTORY
	errorCantResolveFilename syscall.Errno = 1921 // ERROR_CANT_RESOLVE_FILENAME
)

// platformReason returns the fixed reason of the Windows errors that
// fs.ErrPermission and fs.ErrNotExist do not cover, or "". Reading a
// directory's handle as a file fails with ERROR_INVALID_FUNCTION; the os
// package reports a listing of a file as ENOTDIR, which on Windows is
// ERROR_PATH_NOT_FOUND; and an over-long name in an extended-length path
// fails with ERROR_INVALID_NAME, which also means a forbidden character.
func platformReason(err error) string {
	var pathError *fs.PathError
	switch {
	case errors.Is(err, syscall.EISDIR) || errors.Is(err, errorInvalidFunction):
		return IsDirectory
	case errors.Is(err, errorDirectory):
		return NotDirectory
	case errors.As(err, &pathError) && pathError.Op == "readdir" &&
		errors.Is(pathError.Err, errorPathNotFound):
		return NotDirectory
	case errors.Is(err, errorCantResolveFilename):
		return LinkLoop
	case errors.Is(err, errorFilenameExcedRange):
		return NameTooLong
	case errors.As(err, &pathError) && errors.Is(pathError.Err, errorInvalidName) &&
		hasLongComponent(pathError.Path):
		return NameTooLong
	}
	return ""
}

// maxComponent is the longest file or directory name NTFS and the other
// Windows file systems store, in UTF-16 code units.
const maxComponent = 255

// hasLongComponent reports whether a component of path is longer than
// Windows file systems allow.
func hasLongComponent(path string) bool {
	for _, component := range strings.FieldsFunc(path, func(char rune) bool {
		return char == '\\' || char == '/'
	}) {
		units := 0
		for _, char := range component {
			units += utf16.RuneLen(char)
		}
		if units > maxComponent {
			return true
		}
	}
	return false
}
