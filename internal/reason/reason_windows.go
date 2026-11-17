package reason

import (
	"errors"
	"io/fs"
	"syscall"
)

// Windows error codes the syscall package does not name, from the system
// error code list of the Win32 documentation.
const (
	errorInvalidFunction     syscall.Errno = 1    // ERROR_INVALID_FUNCTION
	errorPathNotFound        syscall.Errno = 3    // ERROR_PATH_NOT_FOUND
	errorFilenameExcedRange  syscall.Errno = 206  // ERROR_FILENAME_EXCED_RANGE
	errorDirectory           syscall.Errno = 267  // ERROR_DIRECTORY
	errorCantResolveFilename syscall.Errno = 1921 // ERROR_CANT_RESOLVE_FILENAME
)

// platformReason returns the fixed reason of the Windows errors that
// fs.ErrPermission and fs.ErrNotExist do not cover, or "". Reading a
// directory's handle as a file fails with ERROR_INVALID_FUNCTION, and the
// os package reports a listing of a file as ENOTDIR, which on Windows is
// ERROR_PATH_NOT_FOUND, so only that listing's error is "not a directory".
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
	}
	return ""
}
