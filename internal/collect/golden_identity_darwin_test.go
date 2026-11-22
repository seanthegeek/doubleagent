package collect_test

import "golang.org/x/sys/unix"

// excludedDirectoryHasNoMetadata says an excluded directory's row holds no
// times or owner on Linux and macOS.
const excludedDirectoryHasNoMetadata = true

// pathIdentity returns what lstat reports of path itself, birth time
// included, and whether lstat could describe it.
func pathIdentity(path string) (identity, bool) {
	var stat unix.Stat_t
	if unix.Lstat(path, &stat) != nil {
		return identity{}, false
	}
	return identity{ctime: stat.Ctim.Sec, btime: stat.Btim.Sec, uid: int64(stat.Uid),
		gid: int64(stat.Gid)}, true
}
