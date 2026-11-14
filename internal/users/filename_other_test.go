//go:build !windows

package users

// invalidFileName reports whether err says the file name's syntax is not
// valid on this platform, which only Windows restricts.
func invalidFileName(error) bool {
	return false
}

// onWindows tells the shared tests which platform's expectations apply.
const onWindows = false

// bracketName and bracketRoot are directory names holding glob characters;
// Windows rejects the * of these, so its twins leave it out.
const (
	bracketName = "[x]*"
	bracketRoot = "r[o]*ot"
)
