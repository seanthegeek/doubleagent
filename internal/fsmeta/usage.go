package fsmeta

import (
	"context"
	"os"
)

// dirKey identifies a file by device and inode.
type dirKey struct{ dev, ino uint64 }

// DiskUsageBytes returns what du -sk reports for dir, in bytes: the blocks
// allocated to dir and everything under it, counted in 512-byte units, each
// hard-linked file once, rounded up to whole KiB and multiplied by 1024. A
// symlink counts as itself and is never followed; an unreadable directory
// contributes what could be read, as du does. It is 0 when dir cannot be
// examined. The context is checked before each directory is read, and the
// only error returned is the context's, once it is cancelled.
func DiskUsageBytes(ctx context.Context, dir string) (int64, error) {
	seen := map[dirKey]struct{}{}
	blocks, err := countBlocks(ctx, dir, seen)
	if err != nil {
		return 0, err
	}
	kibibytes := (blocks*512 + 1023) / 1024
	return kibibytes * 1024, nil
}

// countBlocks returns the 512-byte blocks of path and of everything below
// it, leaving out a multiply-linked file seen already holds, or the
// context's error when it is cancelled before a directory is read.
func countBlocks(ctx context.Context, path string, seen map[dirKey]struct{}) (int64, error) {
	meta, err := Lstat(path)
	if err != nil {
		return 0, nil
	}
	if meta.Nlink > 1 {
		key := dirKey{meta.Dev, meta.Ino}
		if _, counted := seen[key]; counted {
			return 0, nil
		}
		seen[key] = struct{}{}
	}
	total := meta.Blocks
	if meta.Kind != Dir {
		return total, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	handle, err := os.Open(path)
	if err != nil {
		return total, nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	for _, entry := range entries {
		blocks, err := countBlocks(ctx, path+"/"+entry.Name(), seen)
		if err != nil {
			return 0, err
		}
		total += blocks
	}
	return total, nil
}
