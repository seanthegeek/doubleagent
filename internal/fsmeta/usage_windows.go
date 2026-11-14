package fsmeta

import (
	"context"
	"os"
)

// DiskUsageBytes returns the size of dir as the PowerShell collector's
// Get-DirSize reports it: the sum of the sizes of the files under dir, each
// path counted once (a file with several hard links is counted under each).
// Only directories are descended; a symlink, junction or other reparse point
// is neither counted nor followed, dir included. An unreadable directory
// contributes what could be read; dir that cannot be examined gives 0. The
// context is checked before each directory is read, and the only error
// returned is the context's, once it is cancelled.
func DiskUsageBytes(ctx context.Context, dir string) (int64, error) {
	meta, err := Lstat(dir)
	if err != nil {
		return 0, nil
	}
	return sumFileSizes(ctx, dir, meta)
}

// sumFileSizes returns the size of path when meta says it is a file, the
// sizes of the files below it when it is a directory, else 0, or the
// context's error when it is cancelled before a directory is read.
func sumFileSizes(ctx context.Context, path string, meta Meta) (int64, error) {
	switch meta.Kind {
	case File:
		return meta.Size, nil
	case Dir:
	default:
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	handle, err := os.Open(path)
	if err != nil {
		return 0, nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	var total int64
	for _, entry := range entries {
		child := path + "/" + entry.Name()
		childMeta, err := Lstat(child)
		if err != nil {
			continue
		}
		size, err := sumFileSizes(ctx, child, childMeta)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}
