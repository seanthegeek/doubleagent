package input

import (
	"errors"
	"syscall"
)

// Windows error codes the syscall package does not name, from the system
// error code list of the Win32 documentation.
const (
	errorInvalidName        syscall.Errno = 123 // ERROR_INVALID_NAME
	errorFilenameExcedRange syscall.Errno = 206 // ERROR_FILENAME_EXCED_RANGE
)

// hostMemberName reports whether Windows stores a member name's component
// as written (windowsNameAllowed).
func hostMemberName(name string) bool { return windowsNameAllowed(name) }

// nameRefused reports whether err is the system refusing a name it cannot
// hold for what it holds: an invalid name.
func nameRefused(err error) bool {
	return errors.Is(err, errorInvalidName)
}

// nameTooLong reports whether err is the system refusing a name for its
// length: a file name or a path longer than the system takes.
func nameTooLong(err error) bool {
	return errors.Is(err, errorFilenameExcedRange)
}
