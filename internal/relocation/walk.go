package relocation

import (
	"context"
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/users"
)

// sweepWalk lists the files of one sweep depth first, each directory's
// entries in the order it lists them, never following a symlink or any other
// reparse point (a Windows junction included), a pruned path left out with
// everything under it.
type sweepWalk struct {
	ctx     context.Context
	opener  *fsmeta.Opener
	pruned  func(path string) bool
	limit   int64
	files   []string
	skipped int
}

// visit handles path: a regular file is listed when it is within the limit
// (0 for none) and counted as skipped otherwise;
// a directory is read and its entries visited. A directory that cannot be
// read gives the entries read before the error, as find goes on past it.
func (walk *sweepWalk) visit(path string) error {
	if walk.pruned(path) {
		return nil
	}
	meta, err := fsmeta.Lstat(path)
	if err != nil {
		return nil
	}
	if meta.Kind == fsmeta.File {
		walk.addFile(path, meta.Size)
		return nil
	}
	if meta.Kind != fsmeta.Dir {
		return nil
	}
	if err := walk.ctx.Err(); err != nil {
		return err
	}
	for _, entry := range readEntries(walk.opener, path) {
		if err := walk.visit(path + "/" + entry.Name()); err != nil {
			return err
		}
	}
	return nil
}

// addFile lists a regular file or counts it as over the size limit.
func (walk *sweepWalk) addFile(path string, size int64) {
	if walk.limit == 0 || size <= walk.limit {
		walk.files = append(walk.files, path)
	} else {
		walk.skipped++
	}
}

// readEntries returns the entries of directory in directory order, listed
// through fsmeta.OpenDir with opener.
func readEntries(opener *fsmeta.Opener, directory string) []fs.DirEntry {
	handle, err := fsmeta.OpenDir(opener, directory)
	if err != nil {
		return nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	return entries
}

// pathsOf returns the paths of a user or project list.
func pathsOf(bases []users.Home) []string {
	paths := make([]string, len(bases))
	for i, base := range bases {
		paths[i] = base.Path
	}
	return paths
}
