package fsmeta

import "golang.org/x/sys/unix"

// Lstat returns the metadata of path itself, birth time included.
func Lstat(path string) (Meta, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return Meta{}, pathError("lstat", path, err)
	}
	return Meta{
		Size:   stat.Size,
		Mtime:  stat.Mtim.Sec,
		Atime:  stat.Atim.Sec,
		Ctime:  stat.Ctim.Sec,
		Btime:  stat.Btim.Sec,
		UID:    stat.Uid,
		GID:    stat.Gid,
		Mode:   modeString(uint32(stat.Mode)),
		Kind:   kindOf(uint32(stat.Mode)),
		Dev:    uint64(uint32(stat.Dev)),
		Ino:    stat.Ino,
		Nlink:  uint64(stat.Nlink),
		Blocks: stat.Blocks,
	}, nil
}
