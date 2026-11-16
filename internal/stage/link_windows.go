package stage

import (
	"errors"
	"os"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// refusesDotted is whether Dest refuses a path with a `.` or `..` component:
// on Windows, where none should be left.
const refusesDotted = true

// allowUnprivilegedCreate is SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE,
// which golang.org/x/sys does not define: with developer mode on, it lets an
// account without the symlink privilege create a link.
const allowUnprivilegedCreate = 0x2

// createLink makes dest a symlink to target, a directory link when directory
// is set, as the PowerShell collector recreates a junction or a directory
// symlink. It calls CreateSymbolicLink itself because os.Symlink picks the
// kind from what the target is now, which a link into the source cannot
// tell. The target is stored as given, never made extended or absolute.
// Windows releases before developer mode reject the unprivileged flag as an
// invalid parameter, so the call is retried without it.
func createLink(target, dest string, directory bool) error {
	linkName, err := windows.UTF16PtrFromString(syspath.Extended(dest))
	if err != nil {
		return linkError(target, dest, err)
	}
	targetName, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return linkError(target, dest, err)
	}
	flags := uint32(allowUnprivilegedCreate)
	if directory {
		flags |= windows.SYMBOLIC_LINK_FLAG_DIRECTORY
	}
	err = windows.CreateSymbolicLink(linkName, targetName, flags)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		err = windows.CreateSymbolicLink(linkName, targetName, flags&^allowUnprivilegedCreate)
	}
	if err != nil {
		return linkError(target, dest, err)
	}
	return nil
}

// linkError returns the *os.LinkError of a failed link creation, naming the
// staged path in native form and the target as stored.
func linkError(target, dest string, err error) error {
	return &os.LinkError{Op: "symlink", Old: target, New: syspath.ToNative(dest), Err: err}
}
