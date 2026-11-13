package stage

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// accessTime returns the access time of a stat result.
func accessTime(status *syscall.Stat_t) unix.Timespec {
	return unix.Timespec{Sec: status.Atimespec.Sec, Nsec: status.Atimespec.Nsec}
}
