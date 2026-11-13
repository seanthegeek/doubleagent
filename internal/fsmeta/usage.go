package fsmeta

import (
	"os"
)

// dirKey identifies a file by device and inode.
type dirKey struct{ dev, ino uint64 }

// DiskUsageBytes returns what du -sk reports for dir, in bytes: the blocks
// allocated to dir and everything under it, counted in 512-byte units, each
// hard-linked file once, rounded up to whole KiB and multiplied by 1024. A
// symlink counts as itself and is never followed; an unreadable directory
// contributes what could be read, as du does. It is 0 when dir cannot be
// examined.
func DiskUsageBytes(dir string) int64 {
	seen := map[dirKey]struct{}{}
	blocks := countBlocks(dir, seen)
	kibibytes := (blocks*512 + 1023) / 1024
	return kibibytes * 1024
}

// countBlocks returns the 512-byte blocks of path and of everything below
// it, leaving out a multiply-linked file seen already holds.
func countBlocks(path string, seen map[dirKey]struct{}) int64 {
	meta, err := Lstat(path)
	if err != nil {
		return 0
	}
	if meta.Nlink > 1 {
		key := dirKey{meta.Dev, meta.Ino}
		if _, counted := seen[key]; counted {
			return 0
		}
		seen[key] = struct{}{}
	}
	total := meta.Blocks
	if meta.Kind != Dir {
		return total
	}
	handle, err := os.Open(path)
	if err != nil {
		return total
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	for _, entry := range entries {
		total += countBlocks(path+"/"+entry.Name(), seen)
	}
	return total
}
