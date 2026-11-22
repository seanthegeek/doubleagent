//go:build !windows

package reason

import (
	"errors"
	"syscall"
)

// platformReason returns the fixed reason of the Unix errors that
// fs.ErrPermission and fs.ErrNotExist do not cover, or "".
func platformReason(err error) string {
	switch {
	case errors.Is(err, syscall.EISDIR):
		return IsDirectory
	case errors.Is(err, syscall.ENOTDIR):
		return NotDirectory
	case errors.Is(err, syscall.ELOOP):
		return LinkLoop
	case errors.Is(err, syscall.ENAMETOOLONG):
		return NameTooLong
	}
	return ""
}
