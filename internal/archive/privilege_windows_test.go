package archive

import (
	"errors"
	"syscall"
)

// errorPrivilegeNotHeld is ERROR_PRIVILEGE_NOT_HELD, which creating a
// symbolic link returns without the privilege to do so.
const errorPrivilegeNotHeld syscall.Errno = 1314

// symlinkNeedsPrivilege reports whether err says the process may not create
// symbolic links.
func symlinkNeedsPrivilege(err error) bool {
	return errors.Is(err, errorPrivilegeNotHeld)
}
