package fsmeta

import (
	"errors"

	"golang.org/x/sys/unix"
)

// statxCall and lstatCall are the system calls Lstat makes, as functions so
// a test can make statx fail the way an old kernel or a filter does.
type (
	statxCall func(dirfd int, path string, flags int, mask int, stat *unix.Statx_t) error
	lstatCall func(path string, stat *unix.Stat_t) error
)

// Lstat returns the metadata of path itself through statx, which also gives
// the birth time on file systems that keep one; Btime is 0 on the others,
// and wherever statx is refused and lstat answers instead.
func Lstat(path string) (Meta, error) {
	return lstatWith(path, unix.Statx, unix.Lstat)
}

// lstatWith is Lstat over the two calls given. statx fails with ENOSYS on a
// kernel older than 4.11 and with EPERM under a seccomp filter (a container
// runtime's list of allowed system calls) written before statx existed; on
// either the same path is retried with lstat, which has no birth time. EPERM
// can also be a real refusal of this path, and lstat then returns that
// refusal. Each call decides afresh; nothing is remembered between calls.
func lstatWith(path string, statx statxCall, lstat lstatCall) (Meta, error) {
	var stx unix.Statx_t
	mask := unix.STATX_BASIC_STATS | unix.STATX_BTIME
	err := statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, mask, &stx)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EPERM) {
		var stat unix.Stat_t
		if err := lstat(path, &stat); err != nil {
			return Meta{}, pathError("lstat", path, err)
		}
		return statMeta(&stat), nil
	}
	if err != nil {
		return Meta{}, pathError("lstat", path, err)
	}
	return statxMeta(&stx), nil
}

// statxMeta fills a Meta from what statx returned.
func statxMeta(stx *unix.Statx_t) Meta {
	meta := Meta{
		Size:   int64(stx.Size),
		Mtime:  stx.Mtime.Sec,
		Atime:  stx.Atime.Sec,
		Ctime:  stx.Ctime.Sec,
		UID:    stx.Uid,
		GID:    stx.Gid,
		Mode:   modeString(uint32(stx.Mode)),
		Kind:   kindOf(uint32(stx.Mode)),
		Dev:    unix.Mkdev(stx.Dev_major, stx.Dev_minor),
		Ino:    stx.Ino,
		Nlink:  uint64(stx.Nlink),
		Blocks: int64(stx.Blocks),
	}
	if stx.Mask&unix.STATX_BTIME != 0 {
		meta.Btime = stx.Btime.Sec
	}
	return meta
}

// statMeta fills a Meta from what lstat returned, with no birth time. The
// conversions cover every Linux layout of Stat_t: the link count and the
// seconds of a time are 32 bits wide on some architectures. The kernel packs
// the device number the way Mkdev does, so Dev equals the statx path's.
func statMeta(stat *unix.Stat_t) Meta {
	return Meta{
		Size:   int64(stat.Size),
		Mtime:  int64(stat.Mtim.Sec),
		Atime:  int64(stat.Atim.Sec),
		Ctime:  int64(stat.Ctim.Sec),
		UID:    stat.Uid,
		GID:    stat.Gid,
		Mode:   modeString(uint32(stat.Mode)),
		Kind:   kindOf(uint32(stat.Mode)),
		Dev:    uint64(stat.Dev),
		Ino:    uint64(stat.Ino),
		Nlink:  uint64(stat.Nlink),
		Blocks: int64(stat.Blocks),
	}
}
