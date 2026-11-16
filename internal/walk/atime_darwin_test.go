package walk

import (
	"io/fs"
	"syscall"
)

// accessTime returns the access time info carries, in seconds.
func accessTime(info fs.FileInfo) int64 {
	return info.Sys().(*syscall.Stat_t).Atimespec.Sec
}
