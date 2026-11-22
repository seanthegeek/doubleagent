package fsmeta

import "context"

// DiskUsageBytes returns the size of the directory tree at dir as the
// platform reports it (usageWalk.size says how), or the
// context's error once ctx is cancelled; the context is checked before each
// directory is read. Each directory is opened through opener, whose Admit
// judges the handle on an image run.
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

// aboutToOpen calls beforeOpen, when set, with the path about to be opened.
func (walk usageWalk) aboutToOpen(path string) {
	if walk.beforeOpen != nil {
		walk.beforeOpen(path)
	}
}
