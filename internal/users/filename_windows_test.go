package users

import "errors"

// invalidFileName reports whether err says the file name's syntax is not
// valid on this platform.
func invalidFileName(err error) bool {
	return errors.Is(err, errorInvalidName)
}

// bracketName and bracketRoot are directory names holding glob characters,
// without the * that Windows rejects in a name.
const (
	bracketName = "[x]"
	bracketRoot = "r[o]ot"
)
