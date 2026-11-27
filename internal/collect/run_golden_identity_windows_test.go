package collect_test

import "github.com/seanthegeek/doubleagent/internal/fsmeta"

// pathIdentity returns what the file system reports of path itself, never a
// link's target: Windows has no change time or owner numbers, so those are
// 0, the birth time is the creation time and the owner the account name the
// security descriptor gives. It reports whether the path could be described.
func pathIdentity(path string) (identity, bool) {
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return identity{}, false
	}
	return identity{btime: meta.Btime, owner: fsmeta.NewOwners().Name(path)}, true
}
