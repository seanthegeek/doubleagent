package users

import (
	"errors"
	"syscall"
)

// errorInvalidName is ERROR_INVALID_NAME, which Windows returns for a name
// holding a character such as *, ? or a newline.
const errorInvalidName syscall.Errno = 123

// invalidFileName reports whether err says the file name's syntax is not
// valid on this platform.
func invalidFileName(err error) bool {
	return errors.Is(err, errorInvalidName)
}

// onWindows tells the shared tests which platform's expectations apply.
const onWindows = true

// bracketName and bracketRoot are directory names holding glob characters,
// without the * that Windows rejects in a name.
const (
	bracketName = "[x]"
	bracketRoot = "r[o]ot"
)
