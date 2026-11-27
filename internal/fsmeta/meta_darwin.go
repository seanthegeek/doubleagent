package fsmeta

import (
	"io/fs"
	"syscall"

	"golang.org/x/sys/unix"
)

// deviceNumber returns macOS's 32-bit device number as Meta.Dev holds it,
// never sign-extended, so every identity made from a stat agrees.
func deviceNumber(dev int32) uint64 {
	return uint64(uint32(dev))
}

// Lstat returns the metadata of path itself, birth time included.
func Lstat(path string) (Meta, error) {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return Meta{}, pathError("lstat", path, err)
	}
	return Meta{
		Size:  stat.Size,
		Mtime: stat.Mtim.Sec,
		Atime: stat.Atim.Sec,
		Ctime: stat.Ctim.Sec,
		Btime: stat.Btim.Sec,
		UID:   stat.Uid,
		GID:   stat.Gid,
		Mode:  modeString(uint32(stat.Mode)),
		Kind:  kindOf(uint32(stat.Mode)),
		Dev:   deviceNumber(stat.Dev),
		Ino:   stat.Ino,
		Nlink: uint64(stat.Nlink),
	}, nil
}

// StatIdentity returns the device and inode info describes, from the stat
// of an open file, in the form Lstat gives them; zero when info carries no
// stat.
func StatIdentity(info fs.FileInfo) [2]uint64 {
	if status, ok := info.Sys().(*syscall.Stat_t); ok {
		return [2]uint64{deviceNumber(status.Dev), status.Ino}
	}
	return [2]uint64{}
}
