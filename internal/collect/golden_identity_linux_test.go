package collect_test

import (
	"errors"

	"golang.org/x/sys/unix"
)

// pathIdentity returns what statx reports of path itself, and whether it
// could describe it. The birth time counts only where the file system keeps
// one; a kernel or filter that refuses statx gives lstat's answer, with none.
func pathIdentity(path string) (identity, bool) {
	var stx unix.Statx_t
	mask := unix.STATX_BASIC_STATS | unix.STATX_BTIME
	err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, mask, &stx)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		var stat unix.Stat_t
		if unix.Lstat(path, &stat) != nil {
			return identity{}, false
		}
		return identity{ctime: int64(stat.Ctim.Sec), uid: int64(stat.Uid), gid: int64(stat.Gid)}, true
	}
	if err != nil {
		return identity{}, false
	}
	found := identity{ctime: stx.Ctime.Sec, uid: int64(stx.Uid), gid: int64(stx.Gid)}
	if stx.Mask&unix.STATX_BTIME != 0 {
		found.btime = stx.Btime.Sec
	}
	return found, true
}
