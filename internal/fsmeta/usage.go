package fsmeta

import "context"

// DiskUsageBytes returns the size of the directory tree at dir: the sum of
// the sizes of the files under it, each path counted once (a file with
// several hard links is counted under each), or the context's error once
// ctx is cancelled; the context is checked before each directory is read.
// Only directories are descended; a link is neither counted nor followed,
// dir included. An unreadable directory contributes what could be read, and
// dir that cannot be examined gives 0. Each directory is opened through
// opener, whose Admit judges the handle on an image run.
func DiskUsageBytes(ctx context.Context, opener *Opener, dir string) (int64, error) {
	return usageWalk{opener: opener}.size(ctx, dir)
}

// usageWalk is one sizing of a directory tree: the run's opener, and
// beforeOpen, which a test sets to change a directory between the lstat
// that typed it and the open that lists it.
type usageWalk struct {
	opener     *Opener
	beforeOpen func(path string)
}

// size returns the size of the tree at dir, as DiskUsageBytes does.
func (walk usageWalk) size(ctx context.Context, dir string) (int64, error) {
	meta, err := Lstat(dir)
	if err != nil {
		return 0, nil
	}
	return walk.sumFileSizes(ctx, dir, meta)
}

// sumFileSizes returns the size of path when meta says it is a file, the
// sizes of the files below it when it is a directory, else 0, or the
// context's error when it is cancelled before a directory is read.
func (walk usageWalk) sumFileSizes(ctx context.Context, path string, meta Meta) (int64, error) {
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
	if walk.beforeOpen != nil {
		walk.beforeOpen(path)
	}
	handle, err := openDirectoryItself(walk.opener, path)
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
		size, err := walk.sumFileSizes(ctx, child, childMeta)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}
