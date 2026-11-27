package walk

import (
	"io/fs"
	"syscall"
)

// accessTime returns the access time info carries, in seconds.
func accessTime(info fs.FileInfo) int64 {
	return int64(info.Sys().(*syscall.Stat_t).Atim.Sec)
}
