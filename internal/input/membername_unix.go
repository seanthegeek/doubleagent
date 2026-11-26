//go:build !windows

package input

import (
	"errors"
	"syscall"
)

// nameRefused reports whether err is the system refusing a name it cannot
// hold for what it holds: (macOS) one that is not valid UTF-8.
func nameRefused(err error) bool {
	return errors.Is(err, syscall.EILSEQ)
}

// nameTooLong reports whether err is the system refusing a name for its
// length: a part longer than the file system stores, or a whole path
// longer than the system's file calls take, which give the same error.
func nameTooLong(err error) bool {
	return errors.Is(err, syscall.ENAMETOOLONG)
}
