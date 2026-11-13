package fsmeta

import "golang.org/x/sys/unix"

// Lstat returns the metadata of path itself through statx, which also gives
// the birth time on file systems that keep one; Btime is 0 on the others.
func Lstat(path string) (Meta, error) {
	var stx unix.Statx_t
	mask := unix.STATX_BASIC_STATS | unix.STATX_BTIME
	err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, mask, &stx)
	if err != nil {
		return Meta{}, pathError("lstat", path, err)
	}
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
	return meta, nil
}
